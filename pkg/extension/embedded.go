package extension

import "embed"

//go:embed extensions/*.yaml
var embeddedFS embed.FS
