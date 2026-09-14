package ui

import (
	"embed"
	"io/fs"
)

// Assets are built by Vite before compiling the release binary.
//
//go:embed all:dist
var assets embed.FS

func Files() fs.FS {
	f, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	return f
}
