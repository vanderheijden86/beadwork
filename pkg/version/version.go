package version

// Version is the current application version.
// This is a var (not const) so it can be overridden at build time via:
//
//	go build -ldflags "-X github.com/vanderheijden86/beadwork/pkg/version.Version=v1.2.3"
var Version = "v0.14.4"
