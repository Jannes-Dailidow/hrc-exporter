package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/jannes-dailidow/hrc-exporter/internal/cli"
)

var version = "dev" // set via -ldflags "-X main.version=..."

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(ctx, buildVersion())
	stop()
	os.Exit(code)
}

// buildVersion prefers the ldflags version, then the module version that
// `go install …@vX` records in the binary.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
