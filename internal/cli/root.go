package cli

import (
	goversion "github.com/caarlos0/go-version"
	"github.com/urfave/cli/v3"
)

func New(info goversion.Info) *cli.Command {
	return &cli.Command{
		Name:        info.Name,
		Usage:       info.Description,
		HideVersion: true,
		Commands: []*cli.Command{
			newCACmd(),
			newGenpassCmd(),
			newVersionCmd(info),
		},
	}
}
