package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/lstig/pki/internal/workspace"
)

// workspacePath returns the workspace directory inside the user's runtime
// dir (/run/user/<uid>), a per-session tmpfs created by systemd-logind — so
// the workspace is RAM-backed and needs no privileges or mount units.
func workspacePath() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set (no login session?)")
	}
	return filepath.Join(dir, "workspace"), nil
}

func newWorkspaceCmd() *cli.Command {
	return &cli.Command{
		Name:  "workspace",
		Usage: "Manage the in-memory (tmpfs) workspace",
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Create the in-memory workspace",
				Action: func(_ context.Context, _ *cli.Command) error {
					dir, err := workspacePath()
					if err != nil {
						return err
					}
					if err := os.Mkdir(dir, 0700); err != nil {
						if errors.Is(err, fs.ErrExist) {
							slog.Info("workspace already up", slog.String("path", dir))
							return nil
						}
						return err
					}
					if err := workspace.Initialize(dir); err != nil {
						return err
					}
					slog.Info("workspace ready", slog.String("path", dir))
					return nil
				},
			},
			{
				Name:  "path",
				Usage: "Print the workspace path",
				Action: func(_ context.Context, cmd *cli.Command) error {
					dir, err := workspacePath()
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.Root().Writer, dir)
					return nil
				},
			},
			{
				Name:  "down",
				Usage: "Remove the in-memory workspace",
				Action: func(_ context.Context, _ *cli.Command) error {
					dir, err := workspacePath()
					if err != nil {
						return err
					}
					if err := os.RemoveAll(dir); err != nil {
						return err
					}
					slog.Info("workspace removed")
					return nil
				},
			},
		},
	}
}
