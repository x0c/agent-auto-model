package main

import (
	"os"

	"github.com/x0c/agent-auto-model/internal/app"
)

// 由 -ldflags 注入。
var version = "2.0.2"

func main() {
	if version != "" {
		app.Version = version
	}
	os.Exit(app.Run(os.Args))
}
