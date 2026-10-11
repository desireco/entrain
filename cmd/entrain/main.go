// Command entrain renders binaural-beat and noise sessions ("trains")
// to WAV, FLAC, and MP3.
//
// Phase 0 skeleton: the render pipeline lands in Phases 1–3.
package main

import (
	"fmt"
	"os"
)

var version = "0.0.0-dev"

const usage = `entrain — binaural beat + noise session renderer

Usage:

  entrain render <session.toml> [-o out.wav]   (coming in Phase 2)
  entrain version

Part of EntrainSpace. Roadmap: docs/migration-plan.md.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("entrain", version)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "entrain: unknown command %q\n\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
}
