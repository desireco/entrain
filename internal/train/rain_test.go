package train

import (
	"testing"
	"time"

	"github.com/desireco/entrain/site/audio"
)

func TestMasteredBedsRetainTheirSamples(t *testing.T) {
	for _, tc := range []struct{ name, asset string }{{"rain-loop", "rain"}, {"pink", "pink"}, {"white", "white"}} {
		s := Soundscape{Background: tc.name, Cue: "none", Duration: 62 * time.Second, CueVolumePercent: 100, CueIntervalMS: 625}
		stream, err := s.PCM()
		if err != nil {
			t.Fatal(err)
		}
		asset, err := audio.Background(tc.asset)
		if err != nil {
			t.Fatal(err)
		}
		for i := range stream.Frames {
			l, r := stream.Next()
			if i < audio.Rate/2 || i >= stream.Frames-audio.Rate/2 {
				continue
			}
			pos := int(2*i) % len(asset)
			if l != asset[pos] || r != asset[pos+1] {
				t.Fatalf("%s changed at frame %d", tc.name, i)
			}
		}
	}
}
