package seq

import (
	"fmt"
	"math"
	"slices"
	"testing"
)

func TestMixBedStereoRates(t *testing.T) {
	for _, rates := range [][2]int{{44100, 44100}, {48000, 48000}, {44100, 48000}, {48000, 44100}} {
		inRate, outRate := rates[0], rates[1]
		for active := range 2 {
			t.Run(fmt.Sprintf("%d-to-%d/channel-%d", inRate, outRate, active), func(t *testing.T) {
				// A fractional second exercises rounding to complete stereo frames.
				inFrames := inRate/4 + 7
				in := make([]float64, 2*inFrames)
				for i := range inFrames {
					in[2*i+active] = 0.25 * math.Sin(2*math.Pi*400*float64(i)/float64(inRate))
				}
				p := newBedPlayer(in, inRate, outRate)
				frames := inFrames * outRate / inRate
				if len(p.x) != 2*frames {
					t.Fatalf("got %d samples, want %d", len(p.x), 2*frames)
				}
				if inRate == outRate && !slices.Equal(p.x, in) {
					t.Fatal("matched-rate bed changed")
				}
				for i := range 2 * frames {
					l, r := p.Next()
					tone, silent := l, r
					if active == 1 {
						tone, silent = r, l
					}
					if silent != 0 {
						t.Fatalf("frame %d: silent channel leaked %g", i, silent)
					}
					// Check pitch and level away from the file edges and loop fade.
					if i >= 100 && i < p.period()-100 {
						want := 0.25 * math.Sin(2*math.Pi*400*float64(i)/float64(outRate))
						if math.Abs(tone-want) > 0.0001 {
							t.Fatalf("frame %d: tone %g, want %g", i, tone, want)
						}
					}
				}
			})
		}
	}
}

func TestMixBedDownsampleRejectsAliasing(t *testing.T) {
	const inRate, outRate = 48000, 44100
	in := make([]float64, 2*inRate/4)
	for i := range len(in) / 2 {
		in[2*i] = math.Sin(2 * math.Pi * 23000 * float64(i) / inRate) // above output Nyquist
		in[2*i+1] = 0.25                                              // independent DC reference, including both file boundaries
	}
	p := newBedPlayer(in, inRate, outRate)
	for i := range len(p.x) / 2 {
		if math.Abs(p.x[2*i+1]-0.25) > 1e-12 {
			t.Fatalf("frame %d: DC level changed to %g", i, p.x[2*i+1])
		}
		if i >= 100 && i < len(p.x)/2-100 && math.Abs(p.x[2*i]) > 0.01 {
			t.Fatalf("frame %d: alias amplitude %g exceeds -40 dBFS", i, p.x[2*i])
		}
	}
}

func TestMixBedShortLoop(t *testing.T) {
	for _, rates := range [][2]int{{44100, 44100}, {44100, 48000}, {48000, 44100}} {
		inRate, outRate := rates[0], rates[1]
		for _, frames := range []int{1, 2} {
			in := []float64{0.25, -0.125, 0.25, -0.125}[:2*frames]
			p := newBedPlayer(in, inRate, outRate)
			wantFrames := max(1, frames*outRate/inRate)
			if len(p.x) != 2*wantFrames {
				t.Errorf("%d frames at %d→%d: got %d frames, want %d", frames, inRate, outRate, len(p.x)/2, wantFrames)
			}
			for range 4 {
				l, r := p.Next()
				if math.Abs(l-0.25) > 1e-12 || math.Abs(r+0.125) > 1e-12 || math.IsNaN(l+r) {
					t.Fatalf("%d frames at %d→%d: got (%g, %g), want (0.25, -0.125)", frames, inRate, outRate, l, r)
				}
			}
		}
	}
}
