//go:build !production

package desktop

import (
	"io/fs"
	"os"
)

// Assets returns the built frontend directory during development.
func Assets() fs.FS { return os.DirFS("desktop/frontend/dist") }
