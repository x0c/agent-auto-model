package main

import (
	"os"

	"github.com/x0c/cursor-mode-model/internal/app"
)

// 由 -ldflags 注入。
var version = "1.0.0"

func main() {
	if version != "" {
		app.Version = version
	}
	os.Exit(app.Run(os.Args))
}
