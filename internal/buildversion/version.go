// Package buildversion identifies both release archives and Go module installs.
package buildversion

import "runtime/debug"

// Resolve prefers a tag supplied by the release build. go install path@tag
// records the module version in the binary instead.
func Resolve(tag string) string {
	if tag != "" {
		return tag
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "0.0.0-dev"
}
