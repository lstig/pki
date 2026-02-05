package cli

import (
	_ "embed"

	goversion "github.com/caarlos0/go-version"
	"github.com/urfave/cli/v3"
)

func New(info goversion.Info) *cli.Command {
	return &cli.Command{
		Name:        info.Name,
		Usage:       info.Description,
		HideVersion: true,
		Commands: []*cli.Command{
			newGenpassCmd(),
			newInitCACommand(),
			newVersionCmd(info),
		},
	}
}
