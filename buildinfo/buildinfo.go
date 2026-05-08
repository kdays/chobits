package buildinfo

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = ""
)

func Text() string {
	return fmt.Sprintf("version=%s\ncommit=%s\nbuild_time=%s\n", Version, Commit, BuildTime)
}

func Fields() map[string]string {
	return map[string]string{
		"version":    Version,
		"commit":     Commit,
		"build_time": BuildTime,
	}
}
