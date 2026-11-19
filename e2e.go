//go:build e2e

// This file is only compiled with `-tags e2e`, for driving the app from
// mygo-automation: it embeds the built frontend itself (instead of the
// `mygo build` bundle) and links the automation plugin. A release build
// leaves both out.
package main

import (
	"embed"
	"io/fs"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
)

//go:embed all:dist
var frontend embed.FS

func init() {
	sub, err := fs.Sub(frontend, "dist")
	if err != nil {
		panic(err)
	}
	mygo.SetFrontend(sub)
}
