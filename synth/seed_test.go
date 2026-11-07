package synth

import "testing"

func TestProceduralSeeds(t *testing.T) {
	for _, kind := range []string{"rain", "wind", "surf"} {
		base, _ := NewProcedural(kind, 44100)
		zero, _ := NewSeededProcedural(kind, 44100, 0)
		variant, _ := NewSeededProcedural(kind, 44100, 2147483647)
		repeat, _ := NewSeededProcedural(kind, 44100, 2147483647)
		different := false
		for range 44100 {
			l, r := base.Next()
			zl, zr := zero.Next()
			vl, vr := variant.Next()
			rl, rr := repeat.Next()
			if l != zl || r != zr || vl != rl || vr != rr {
				t.Fatalf("%s: nondeterministic seed", kind)
			}
			different = different || vl != l || vr != r
		}
		if !different {
			t.Fatalf("%s: seed had no effect", kind)
		}
	}
}
