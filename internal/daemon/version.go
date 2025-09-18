package daemon

var version = "dev"

// Version returns the build identifier for the daemon. Overridden via -ldflags at build time.
func Version() string {
	return version
}
