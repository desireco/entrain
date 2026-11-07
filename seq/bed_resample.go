package seq

import "math"

// A windowed sinc interpolates each channel and filters out frequencies
// above the output Nyquist limit when downsampling. Tabulating the kernel
// avoids evaluating trigonometric functions for every tap on long beds.
const (
	bedFilterRadius = 32
	bedFilterSteps  = 512 // table entries per source sample at unit cutoff
)

var bedKernel = func() [bedFilterRadius*bedFilterSteps + 2]float64 {
	var table [bedFilterRadius*bedFilterSteps + 2]float64
	table[0] = 1
	for i := 1; i < bedFilterRadius*bedFilterSteps; i++ {
		x := math.Pi * float64(i) / bedFilterSteps
		window := 0.42 + 0.5*math.Cos(x/bedFilterRadius) + 0.08*math.Cos(2*x/bedFilterRadius)
		table[i] = math.Sin(x) / x * window
	}
	return table
}()

func resampleBed(in []float64, inRate, outRate int) []float64 {
	inFrames := len(in) / 2
	// A nonempty loop shorter than one output frame still has a sample to play.
	outFrames := max(1, int(int64(inFrames)*int64(outRate)/int64(inRate)))
	out := make([]float64, 2*outFrames)
	cutoff := 1.0
	if outRate < inRate {
		// Leave room for the filter transition below the new Nyquist limit.
		cutoff = 0.94 * float64(outRate) / float64(inRate)
	}
	radius := bedFilterRadius / cutoff
	for i := range outFrames {
		pos := float64(i) * float64(inRate) / float64(outRate)
		start := max(0, int(math.Ceil(pos-radius)))
		end := min(inFrames-1, int(math.Floor(pos+radius)))
		var left, right, sum float64
		for frame := start; frame <= end; frame++ {
			x := math.Abs(pos-float64(frame)) * cutoff * bedFilterSteps
			k := int(x)
			weight := bedKernel[k] + (x-float64(k))*(bedKernel[k+1]-bedKernel[k])
			left += weight * in[2*frame]
			right += weight * in[2*frame+1]
			sum += weight
		}
		// Normalize the available taps so constant levels are preserved,
		// including at the endpoints where the window extends past the file.
		out[2*i], out[2*i+1] = left/sum, right/sum
	}
	return out
}
