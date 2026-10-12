// Package beatscope detects binaural beats in stereo audio: it finds the
// dominant spectral peaks per ear, pairs peaks across channels that sit a
// plausible beat distance apart, and reports carrier, beat rate, and a
// confidence score per pair. It is both a user-facing trust tool
// (cmd/beatscope) and the Phase 4 cross-engine validation harness.
package beatscope

import (
	"math"
	"sort"
	"time"

	"github.com/desireco/entrain/internal/dsp"
)

// Analysis parameters. The STFT window trades frequency resolution
// (0.67 Hz at 2^16/44.1k) against time resolution; binaural sessions are
// steady-state, so long windows fit.
const (
	windowSize = 1 << 16
	hop        = windowSize / 2

	minFreq     = 20   // Hz — analysis band
	maxFreq     = 2000 // Hz
	minProm     = 8    // ×band median — a real tone stands out this much
	maxBeat     = 40   // Hz — anything wider is not an entrainment beat
	minBeat     = 0.1  // Hz
	maxLevelGap = 12.0 // dB between paired L/R peaks
)

// Beat is one detected binaural pair.
type Beat struct {
	Carrier    float64       // (left + right) / 2, Hz
	Beat       float64       // |left − right|, Hz
	Level      float64       // vote weight relative to the strongest pair
	Confidence float64       // 0–1: vote share + analyzed duration
	Span       time.Duration // analyzed duration
}

// cand is one per-window spectral peak candidate.
type cand struct {
	freq, mag float64
}

// Analyze detects binaural beats in interleaved stereo audio at the given
// sample rate.
func Analyze(x []float64, sr int) []Beat {
	l, r := dsp.Deinterleave(x)
	if len(l) < windowSize+hop {
		return nil // need at least a window plus one hop
	}

	// Per-window candidate peaks per channel.
	var lcands, rcands [][]cand
	for w := 0; w*hop+windowSize <= len(l); w++ {
		lcands = append(lcands, topPeaks(dsp.Spectrum(l[w*hop:w*hop+windowSize]), sr))
		rcands = append(rcands, topPeaks(dsp.Spectrum(r[w*hop:w*hop+windowSize]), sr))
	}

	// Accumulate votes for (fL, fR) pairs quantized to 0.1 Hz across
	// windows. Binaural pairs are stable — present in every window at
	// the same frequencies — while coincidental noise matches flicker;
	// the per-key window count is the evidence, weight only ranks.
	type pairKey struct{ l, r int } // frequencies ×10
	type tally struct {
		weight  float64
		windows int
	}
	votes := map[pairKey]*tally{}
	for w := range lcands {
		seen := map[pairKey]bool{}
		for _, pl := range lcands[w] {
			for _, pr := range rcands[w] {
				beat := math.Abs(pl.freq - pr.freq)
				if beat < minBeat || beat > maxBeat {
					continue
				}
				gap := 20 * math.Log10(pl.mag/pr.mag)
				if math.Abs(gap) > maxLevelGap {
					continue
				}
				k := pairKey{int(math.Round(pl.freq * 10)), int(math.Round(pr.freq * 10))}
				tv := votes[k]
				if tv == nil {
					tv = &tally{}
					votes[k] = tv
				}
				tv.weight += math.Min(pl.mag, pr.mag)
				seen[k] = true
			}
		}
		for k := range seen {
			votes[k].windows++
		}
	}
	if len(votes) == 0 {
		return nil
	}

	// Rank keys by accumulated vote weight.
	keys := make([]pairKey, 0, len(votes))
	for k := range votes {
		keys = append(keys, k)
	}
	totalWindows := float64(len(lcands))
	sort.Slice(keys, func(i, j int) bool { return votes[keys[i]].weight > votes[keys[j]].weight })

	// Suppress keys that belong to the same physical pair (rounding).
	maxMag := votes[keys[0]].weight
	var beats []Beat
	for _, k := range keys {
		fl, fr := float64(k.l)/10, float64(k.r)/10
		beat := math.Abs(fl - fr)
		carrier := (fl + fr) / 2
		dup := false
		for _, b := range beats {
			if math.Abs(b.Carrier-carrier) < 1 && math.Abs(b.Beat-beat) < 1 {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		tv := votes[k]
		beats = append(beats, Beat{
			Carrier:    carrier,
			Beat:       beat,
			Level:      tv.weight / maxMag,
			Confidence: confidence(float64(tv.windows)/totalWindows, tv.weight/maxMag),
			Span:       time.Duration(float64(len(l)) / float64(sr) * float64(time.Second)),
		})
		if len(beats) == 8 {
			break
		}
	}
	return beats
}

// confidence weighs how consistently the pair appears across STFT
// windows (the real evidence) with its share of the vote weight.
func confidence(windowFraction, weightShare float64) float64 {
	return math.Min(1, 0.75*windowFraction+0.25*weightShare)
}

// topPeaks returns the strongest few analysis-band peaks of a spectrum.
func topPeaks(mag []float64, sr int) []cand {
	pk := dsp.Peaks(mag, float64(sr), minFreq, maxFreq, minProm)
	out := make([]cand, 0, 4)
	for i, p := range pk {
		if i == 4 {
			break
		}
		out = append(out, cand{freq: p.Freq, mag: p.Mag})
	}
	return out
}
