// Package version holds archy's build-time version metadata.
//
// The exported variables are populated by the linker via -ldflags
// at release-build time (see .goreleaser.yaml). In development
// builds — `go run`, `go build` without ldflags — they retain their
// default "dev" / "unknown" values, which is the right behavior for
// local work.
package version

// Version is the semantic version of the binary, e.g. "v0.1.0".
// Set at link time via -X. Defaults to "dev" for development builds.
var Version = "dev"

// Commit is the git commit SHA the binary was built from. Set at
// link time via -X. Defaults to "unknown" for development builds.
var Commit = "unknown"

// Date is the RFC3339 build timestamp. Set at link time via -X.
// Defaults to "unknown" for development builds.
var Date = "unknown"

// String returns a single-line human-readable summary of the
// version metadata, suitable for `archy version` output and for
// inclusion in `archy doctor` (when that command lands).
func String() string {
	return "archy " + Version + " (" + Commit + ", built " + Date + ")"
}
