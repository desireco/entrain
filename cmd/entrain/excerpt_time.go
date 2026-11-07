package main

import (
	"fmt"
	"strings"
	"time"
)

// Native source positions and lengths accept Go durations and the existing
// h:mm[:ss] form, whose two-field version means hours and minutes.
func parseNativeTime(text string) (time.Duration, error) {
	spec := text
	if strings.Contains(text, ":") {
		parts := strings.Split(text, ":")
		if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || strings.Trim(parts[0], "0123456789") != "" {
			return 0, fmt.Errorf("bad clock %q (use h:mm or h:mm:ss)", text)
		}
		spec = parts[0] + "h"
		for i, p := range parts[1:] {
			if len(p) != 2 || strings.Trim(p, "0123456789") != "" || p > "59" {
				return 0, fmt.Errorf("bad clock %q (use h:mm or h:mm:ss)", text)
			}
			spec += p + []string{"m", "s"}[i]
		}
	}
	d, err := time.ParseDuration(spec)
	if err != nil {
		return 0, fmt.Errorf("bad time %q (use a duration such as 30s or h:mm[:ss])", text)
	}
	return d, nil
}
