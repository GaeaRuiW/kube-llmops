package main

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var staticFS embed.FS

func getStaticFS() (fs.FS, error) {
	return fs.Sub(staticFS, "web/dist")
}
