package version

import "strings"

// Build-time metadata populated via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// IsDev reports whether the binary was built without a release version tag.
func IsDev() bool {
	v := strings.TrimSpace(Version)
	return v == "" || v == "dev"
}

// IsPrerelease reports whether the current version is a SemVer pre-release tag (e.g. v0.2.0-dev.1).
func IsPrerelease() bool {
	return !IsDev() && strings.Contains(Version, "-")
}

// Display returns a human-readable version string, defaulting to "dev" when empty.
func Display() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		return "dev"
	}
	return v
}
