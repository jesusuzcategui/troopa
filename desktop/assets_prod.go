//go:build production

package desktop

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var distFS embed.FS

// Assets returns the embedded production frontend bundle.
func Assets() fs.FS { return distFS }
