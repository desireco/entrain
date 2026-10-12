package dsp

import (
	"math"
	"sort"
)

// FFT computes the DFT of re/im in place (iterative radix-2
// Cooley–Tukey). len must be a power of two; panics otherwise.
func FFT(re, im []float64) {
	n := len(re)
	if n == 0 || n&(n-1) != 0 {
		panic("dsp: FFT length must be a power of two")
	}
	if len(im) != n {
		panic("dsp: re/im length mismatch")
	}

	// Bit-reversal permutation.
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}

	// Butterflies.
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		for i := 0; i < n; i += length {
			for k := 0; k < length/2; k++ {
				wr := math.Cos(ang * float64(k))
				wi := math.Sin(ang * float64(k))
				u := i + k
				v := i + k + length/2
				tr := re[v]*wr - im[v]*wi
				ti := re[v]*wi + im[v]*wr
				re[v] = re[u] - tr
				im[v] = im[u] - ti
				re[u] += tr
				im[u] += ti
			}
		}
	}
}

// Spectrum returns the magnitude spectrum of x (bins 0…len(x)/2) after a
// Hann window, normalized so a full-scale sine sums to its bin magnitude
// ×2/N scaling: an A-peak sine yields ≈A·N/4 at its (windowed) peak bin.
// len(x) must be a power of two.
func Spectrum(x []float64) []float64 {
	n := len(x)
	if n == 0 || n&(n-1) != 0 {
		panic("dsp: Spectrum length must be a power of two")
	}
	re := make([]float64, n)
	im := make([]float64, n)
	for i, v := range x {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n)) // Hann
		re[i] = v * w
	}
	FFT(re, im)
	mag := make([]float64, n/2+1)
	for i := range mag {
		mag[i] = math.Hypot(re[i], im[i])
	}
	return mag
}

// Peak is one spectral maximum.
type Peak struct {
	Freq       float64 // interpolated frequency in Hz
	Mag        float64 // spectral magnitude
	Prominence float64 // Mag / band median — how far it stands out
}

// Peaks finds local maxima of a magnitude spectrum (as returned by
// Spectrum) within [minF, maxF], with parabolic interpolation for
// sub-bin frequency accuracy. Peaks are returned in descending
// magnitude order, filtered to prominence ≥ minProm. Prominence is
// measured against the local one-octave median, so sloped spectra
// (pink/brown noise) do not make their own low end look like peaks.
func Peaks(mag []float64, sr, minF, maxF, minProm float64) []Peak {
	// mag holds the half-spectrum of an N-sample FFT (N = 2·(len−1));
	// bin k sits at k·sr/N.
	n := 2 * (len(mag) - 1)
	lo, hi := int(minF*float64(n)/sr), int(maxF*float64(n)/sr)
	if lo < 1 {
		lo = 1
	}
	if hi > n-2 {
		hi = n - 2
	}
	if lo > hi {
		return nil
	}

	var peaks []Peak
	for i := lo; i <= hi; i++ {
		if mag[i] <= mag[i-1] || mag[i] < mag[i+1] {
			continue
		}
		// Local one-octave median around the candidate.
		nlo, nhi := i/2, 2*i
		if nlo < lo {
			nlo = lo
		}
		if nhi > hi {
			nhi = hi
		}
		if nhi-nlo < 4 { // tiny neighborhoods cannot judge prominence
			continue
		}
		prom := mag[i] / median(mag[nlo:nhi+1])
		if prom < minProm {
			continue
		}
		// Parabolic interpolation around the bin maximum.
		denom := mag[i-1] - 2*mag[i] + mag[i+1]
		delta := 0.0
		if denom != 0 {
			delta = 0.5 * (mag[i-1] - mag[i+1]) / denom
		}
		if delta < -1 || delta > 1 {
			delta = 0
		}
		freq := (float64(i) + delta) * sr / float64(n)
		peaks = append(peaks, Peak{Freq: freq, Mag: mag[i], Prominence: prom})
	}
	// Sort by magnitude, descending.
	for i := 1; i < len(peaks); i++ {
		for j := i; j > 0 && peaks[j].Mag > peaks[j-1].Mag; j-- {
			peaks[j], peaks[j-1] = peaks[j-1], peaks[j]
		}
	}
	return peaks
}

func median(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	s := append([]float64(nil), x...)
	sort.Float64s(s)
	return s[len(s)/2]
}
