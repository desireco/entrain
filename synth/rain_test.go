package synth

import (
	"math"
	"testing"
)

func TestRainIndependentOfPullSize(t *testing.T) {
	const sr = 44100
	want := Render(NewRain(sr), 2*sr)
	stream := NewRain(sr)
	offset := 0
	for chunk := 0; offset < len(want); chunk++ {
		n := min([]int{1, 73, 1152, 4096}[chunk%4], (len(want)-offset)/2)
		got := Render(stream, n)
		for i, s := range got {
			if s != want[offset+i] {
				t.Fatalf("chunk %d diverged at sample %d", chunk, offset+i)
			}
		}
		offset += len(got)
	}
}

func TestRainDoesNotRepeatAfterOneMinute(t *testing.T) {
	const sr = 44100
	stream := NewRain(sr)
	first := make([]float64, sr)
	var product, a2, b2 float64
	var previous, boundaryJump, localPower float64
	for i := range 62 * sr {
		left, _ := stream.Next()
		if math.IsNaN(left) || math.Abs(left) > 0.35 {
			t.Fatalf("invalid rain at frame %d: %g", i, left)
		}
		if i >= sr && i < 2*sr {
			first[i-sr] = left
		}
		if i >= 61*sr {
			a := first[i-61*sr]
			product += a * left
			a2 += a * a
			b2 += left * left
		}
		delta := left - previous
		if i == 60*sr {
			boundaryJump = math.Abs(delta)
		}
		if i >= 60*sr-sr/4 && i < 60*sr+sr/4 {
			localPower += delta * delta
		}
		previous = left
	}
	corr := product / math.Sqrt(a2*b2)
	if math.IsNaN(corr) || math.Abs(corr) > .15 {
		t.Fatalf("one-minute repeat correlation %.3f", corr)
	}
	if boundaryJump > 6*math.Sqrt(localPower/(sr/2.0)) {
		t.Fatalf("discontinuity at one minute: %g", boundaryJump)
	}
}

func TestRainSimilarLevelAndWidthAtSupportedRates(t *testing.T) {
	var previousRMS, previousCorr float64
	for _, rate := range []int{44100, 48000} {
		stream := NewRain(float64(rate))
		var l2, r2, lr float64
		n := rate * 5
		for range n {
			l, r := stream.Next()
			l2 += l * l
			r2 += r * r
			lr += l * r
		}
		rms := math.Sqrt((l2 + r2) / float64(2*n))
		corr := lr / math.Sqrt(l2*r2)
		if rms < .025 || rms > .07 || corr < .15 || corr > .35 {
			t.Fatalf("%d Hz: unexpected level %g or stereo correlation %g", rate, rms, corr)
		}
		if previousRMS != 0 && (math.Abs(rms/previousRMS-1) > .15 || math.Abs(corr-previousCorr) > .07) {
			t.Fatalf("level or width changed with rate: (%g,%g) vs (%g,%g)", rms, corr, previousRMS, previousCorr)
		}
		previousRMS, previousCorr = rms, corr
	}
}
