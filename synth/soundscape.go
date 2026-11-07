package synth

// The CLI and browser worker select continuous generators at the same rate;
// mastered loops (pink, white and rain-loop) stay on their original path.
const MaxSoundscapeSeed = 2147483647

// A distinct user seed changes each component's PRNG stream. Zero leaves
// every established source seed (and hence the default waveform) untouched.
func streamSeed(base uint64, seed int64) uint64 {
	return base ^ (uint64(seed) * 0x9E3779B97F4A7C15)
}

var procedural = map[string]func(float64, int64) StereoVoice{
	"rain": func(sr float64, seed int64) StereoVoice { return newRain(sr, seed) },
	"wind": func(sr float64, seed int64) StereoVoice { return newWind(sr, seed) },
	"surf": func(sr float64, seed int64) StereoVoice { return newSurf(sr, seed) },
}

func IsProcedural(name string) bool {
	_, ok := procedural[name]
	return ok
}

func NewProcedural(name string, sr float64) (StereoVoice, bool) {
	return NewSeededProcedural(name, sr, 0)
}

func NewSeededProcedural(name string, sr float64, seed int64) (StereoVoice, bool) {
	newVoice, ok := procedural[name]
	if !ok || seed < 0 || seed > MaxSoundscapeSeed {
		return nil, false
	}
	return newVoice(sr, seed), true
}
