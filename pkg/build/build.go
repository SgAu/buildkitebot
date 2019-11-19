package build

import (
	"runtime"
	"strings"
	"time"
)

// These values are overridden by -ldflags.
var (
	Name            = "unknown"
	Version         = "unknown"
	GitCommit       = "unknown"
	BuildTime       = "unknown"
	OperatingSystem = runtime.GOOS
	Architecture    = runtime.GOARCH
)

type Info struct {
	Name            string
	Version         string
	GoVersion       string
	GitCommit       string
	BuildTime       string
	OperatingSystem string
	Architecture    string
}

func GetInfo() *Info {
	return &Info{
		Name:            Name,
		Version:         Version,
		GoVersion:       runtime.Version(),
		GitCommit:       GitCommit,
		BuildTime:       formattedBuildTime(),
		OperatingSystem: OperatingSystem,
		Architecture:    Architecture,
	}
}

// formattedBuildTime returns the build time formatted as a UNIX date -
// e.g. "Tue Mar 12 23:41:36 +0000 2019"
func formattedBuildTime() string {
	t, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		panic(err)
	}

	// On Darwin architectures "+0000" is returned instead of "UTC" so replace for consistency
	return strings.ReplaceAll(t.Format(time.UnixDate), "+0000", "UTC")
}
