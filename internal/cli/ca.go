package cli

import (
	"github.com/urfave/cli/v3"
)

func newCACmd() *cli.Command {
	return &cli.Command{
		Name:  "ca",
		Usage: "Certificate authority utilities",
		Commands: []*cli.Command{
			newCAInitCmd(),
		},
	}
}
