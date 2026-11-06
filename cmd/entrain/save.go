package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/desireco/entrain/internal/train"
)

// cmdSaveSession is the source-only mode of mix. A newly published train is
// independent of this directory and has no audio-side effects.
func cmdSaveSession(sound train.Soundscape, cfg mixOutput, path string, stdout, stderr io.Writer) int {
	if cfg.Path != "" {
		fmt.Fprintln(stderr, "entrain: --save-session writes source only; use -o with entrain render")
		return 2
	}
	if !isNative(path) {
		fmt.Fprintln(stderr, "entrain: --save-session requires a .entrain or .toml filename")
		return 2
	}
	if cfg.Format == "" {
		cfg.Format = "wav"
	}
	session := train.Session{Output: cfg.Format, Soundscape: sound, BitDepth: cfg.Bits, MP3Bitrate: cfg.Bitrate}
	data, err := train.MarshalMix(session)
	if err != nil {
		fmt.Fprintln(stderr, "entrain:", err)
		return 2
	}
	if cfg.DryRun {
		parent, err := os.Stat(filepath.Dir(path))
		if err != nil {
			fmt.Fprintln(stderr, "entrain:", err)
			return 1
		}
		if !parent.IsDir() {
			fmt.Fprintln(stderr, "entrain: destination parent must be a directory")
			return 1
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				err = fmt.Errorf("%s already exists", path)
			}
			fmt.Fprintln(stderr, "entrain:", err)
			return 1
		}
		fmt.Fprintf(stdout, "would save %s\n%s", path, data)
		return 0
	}
	if err := writeNewSession(path, data); err != nil {
		fmt.Fprintln(stderr, "entrain:", err)
		return 1
	}
	fmt.Fprintf(stdout, "saved %s\n", path)
	return 0
}

// Linking a fully written sibling into place publishes the whole file in a
// single create-only operation, even when two mix invocations race or the
// destination is a dangling symlink. A failing writer cannot truncate or
// remove another writer's file.
func writeNewSession(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".entrain-source-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Link(f.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; choose a new session filename", path)
		}
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
