package profile

import (
	"embed"
)

//go:embed all:embedded_profiles/*.json
var embeddedProfilesFS embed.FS
