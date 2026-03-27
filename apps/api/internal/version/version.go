package version

// Version is injected at build time. Defaults to "dev" for local development.
// Production builds set this via: go build -ldflags "-X .../version.Version=v1.0.0"
var Version = "dev"

const (
	// Brand
	Name    = "Sailbox"
	Website = "https://github.com/sailboxhq/sailbox"
	License = "AGPL-3.0"

	// GitHub
	GitHubOwner = "sailboxhq"
	GitHubRepo  = "sailbox"
)
