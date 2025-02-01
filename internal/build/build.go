package build

import "strings"

var (
	Version = ""
	AppName = "Dagu"
	Slug    = ""
)

func init() {
	if Slug == "" {
		Slug = strings.ToLower(AppName)
	}
}
