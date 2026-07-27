package cli

import (
	"context"
	"fmt"

	goversion "github.com/caarlos0/go-version"
	"github.com/urfave/cli/v3"
)

func newVersionCmd(info goversion.Info) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print version information",
		Action: func(_ context.Context, cmd *cli.Command) error {
			fmt.Fprint(cmd.Root().Writer, info.String())
			return nil
		},
	}
}
