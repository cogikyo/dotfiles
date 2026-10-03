package gobuild

var (
	Flags = []string{"-trimpath", "-buildvcs=false"}
	Env   = []string{"CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off"}
)
