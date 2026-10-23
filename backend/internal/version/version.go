// Package version holds build-time version metadata for PrivacyOS.
package version

// Version is the PrivacyOS release version. It can be overridden at build time:
//
//	go build -ldflags "-X privacyos/backend/internal/version.Version=1.2.3"
var Version = "1.1.0"

// BuildTime and Commit are injected by the release pipeline.
var (
	BuildTime = "unknown"
	Commit    = "unknown"
)
