package train

import (
	"fmt"
	"time"

	"github.com/desireco/entrain/codec"
)

// Excerpt is a half-open window into the original, fully authored session.
type Excerpt struct {
	Start, End         time.Duration
	FirstFrame, Frames int64
	session            Soundscape
}

// Excerpt plans the window without generating audio (also used by dry-run).
// A missing length selects the remainder. The end can never exceed the
// number of complete frames in the original render.
func (s Soundscape) Excerpt(start time.Duration, length *time.Duration) (Excerpt, error) {
	if start < 0 || start >= s.Duration {
		return Excerpt{}, fmt.Errorf("excerpt start must be within [0, %s)", s.Duration)
	}
	end := s.Duration
	if length != nil {
		if *length <= 0 {
			return Excerpt{}, fmt.Errorf("excerpt length must be positive")
		}
		// Compare before adding to avoid overflow on a very large limit.
		end = start + min(*length, s.Duration-start)
	}
	first := firstFrame(start)
	stop := min(firstFrame(end), frames(s.Duration))
	if stop <= first {
		return Excerpt{}, fmt.Errorf("excerpt contains no audio frames")
	}
	return Excerpt{Start: start, End: end, FirstFrame: first, Frames: stop - first, session: s}, nil
}

// PCM warms the original stream to the excerpt start. This preserves
// procedural filter state, cue phase and original fades in bounded memory.
func (e Excerpt) PCM() (codec.StereoPCM, error) {
	pcm, err := e.session.PCM()
	if err != nil {
		return codec.StereoPCM{}, err
	}
	for range e.FirstFrame {
		pcm.Next()
	}
	pcm.StartFrame = e.FirstFrame
	pcm.Frames = e.Frames
	return pcm, nil
}
