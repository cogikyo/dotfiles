// Package gobuild shares settings across CLI updates, hyprd rebuilds, and ISO builds so byte comparisons exclude paths and VCS state.
package gobuild

var (
	Flags = []string{"-trimpath", "-buildvcs=false"}
	Env   = []string{"CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off"}
)
