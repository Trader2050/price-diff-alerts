package version

var (
	// Version is the semantic version of the binary. Overridden at build time.
	Version = "dev"
	// Commit is the git commit hash. Overridden at build time.
	Commit = "unknown"
	// BuildDate is the build timestamp. Overridden at build time.
	BuildDate = "unknown"
)
