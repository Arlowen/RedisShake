package webassets

import (
	"embed"
	"io/fs"
)

// content is populated by the Web build before redis-shake-server is compiled.
// The placeholder keeps normal Go builds valid when the frontend has not been built.
//
//go:embed all:dist
var content embed.FS

func FileSystem() (fs.FS, bool) {
	web, err := fs.Sub(content, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(web, "index.html"); err != nil {
		return nil, false
	}
	return web, true
}
