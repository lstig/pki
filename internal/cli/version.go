package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	goversion "github.com/caarlos0/go-version"
	"github.com/urfave/cli/v3"
)

func newVersionCmd(info goversion.Info) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "print version information",
		Action: func(_ context.Context, _ *cli.Command) error {
			fmt.Fprintf(os.Stderr, info.String())
			return nil
		},
	}
}
