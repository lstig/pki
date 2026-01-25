package main

import (
	"context"
	_ "embed"
	"fmt"

	goversion "github.com/caarlos0/go-version"
	"github.com/urfave/cli/v3"
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

func newVersionCmd() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print version information",
		Action: func(_ context.Context, _ *cli.Command) error {
			info := goversion.GetVersionInfo(
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
			fmt.Println(info.String())
			return nil
		},
	}
}
