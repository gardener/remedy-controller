package version

import (
	"fmt"
	"runtime"
)

var (
	gitVersion = "0.0.0-dev"
	gitCommit  string
	buildDate  = "1970-01-01T00:00:00Z"
)

// Get returns the overall codebase version.
func Get() string {
	return fmt.Sprintf(`git version : %s
git commit  : %s
build date  : %s
go version  : %s
go compiler : %s
platform    : %s/%s`, gitVersion, gitCommit, buildDate, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
}
