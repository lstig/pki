package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	goversion "github.com/caarlos0/go-version"
	"github.com/urfave/cli/v3"
)

// New builds the root command. Writer and ErrWriter are set here and nowhere
// else: subcommands inherit both, so actions address the streams as
// cmd.Root().Writer (the output you would pipe) and cmd.Root().ErrWriter
// (warnings, progress, errors) rather than reaching for os.Stdout directly.
func New(info goversion.Info) *cli.Command {
	root := &cli.Command{
		Name:        info.Name,
		Usage:       info.Description,
		HideVersion: true,
		Writer:      os.Stdout,
		ErrWriter:   os.Stderr,
		Commands: []*cli.Command{
			newCACmd(),
			newLUKSCmd(),
			newWorkspaceCmd(),
			newGenpassCmd(),
			newVersionCmd(info),
		},
	}
	setUsageError(root)
	return root
}

// setUsageError installs usageError on cmd and every subcommand. Unlike the
// writers, OnUsageError is read off the failing command and is not inherited
// from the parent, so it has to be set on each one.
func setUsageError(cmd *cli.Command) {
	cmd.OnUsageError = usageError
	for _, sub := range cmd.Commands {
		setUsageError(sub)
	}
}

// usageError returns a flag parsing error for the caller to print. Without it
// urfave splits the failure across both streams: the "Incorrect Usage" line
// goes to ErrWriter while the help text it prints goes to Writer, so a
// mistyped flag lands help output in a redirected stdout.
func usageError(_ context.Context, cmd *cli.Command, err error, _ bool) error {
	return fmt.Errorf("%w\nsee '%s --help' for usage", err, strings.Join(cmd.Path(), " "))
}
