package main

import (
	"context"
	"os"

	"github.com/temirov/ghttp/internal/app"
)

func main() {
	exitCode := app.Execute(context.Background(), os.Args[1:])
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
