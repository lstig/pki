package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/signal"

	goversion "github.com/caarlos0/go-version"

	"github.com/lstig/pki/internal/cli"
)

var (
	version   = "not-built-correctly"
	commit    = "not-built-correctly"
	treeState = "not-built-correctly"
	date      = "not-built-correctly"
	builtBy   = "not-built-correctly"
	//go:embed ascii.txt
	ascii string
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()
	if err := cli.New(versionInfo()).Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func versionInfo() goversion.Info {
	return goversion.GetVersionInfo(
		goversion.WithAppDetails("pki", "Air-gapped PKI helper utility", "https://github.com/lstig/pki"),
		goversion.WithBuiltBy(builtBy),
		goversion.WithASCIIName(ascii),
		func(i *goversion.Info) {
			i.GitCommit = version
			i.GitCommit = commit
			i.GitTreeState = treeState
			i.BuildDate = date
		},
	)
}
