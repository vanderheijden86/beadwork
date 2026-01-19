package version

// Version is the current application version.
// This is a var (not const) so it can be overridden at build time via:
//
//	go build -ldflags "-X github.com/Dicklesworthstone/beads_viewer/pkg/version.Version=v1.2.3"
var Version = "v0.13.1"
