package webembed

import (
	"embed"
	"io/fs"
)

// files contains Vite output. The all: prefix keeps the placeholder file available before the first web build.
//
//go:embed all:dist
var files embed.FS

// FileSystem returns the embedded Vite output as an fs.FS rooted at dist.
func FileSystem() fs.FS {
	web, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return web
}
