package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/urfave/cli/v3"
)

const (
	// WorkspacePath is where the in-memory tmpfs workspace is mounted.
	WorkspacePath = "/mnt/workspace"
	// WorkspaceUnit is the systemd mount unit backing the workspace.
	WorkspaceUnit = "mnt-workspace.mount"
)

func newWorkspaceCmd() *cli.Command {
	return &cli.Command{
		Name:  "workspace",
		Usage: "Manage the in-memory (tmpfs) workspace",
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Mount the in-memory workspace",
				Action: withClient(func(ctx context.Context, _ *cli.Command, conn *systemd.Conn) error {
					done := make(chan string, 1)
					if _, err := conn.StartUnitContext(ctx, WorkspaceUnit, "replace", done); err != nil {
						return fmt.Errorf("could not mount workspace: %w", err)
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case result := <-done:
						if result != "done" {
							return fmt.Errorf("%s did not start successfully, finished with state: %s", WorkspaceUnit, result)
						}
					}
					slog.Info("workspace mounted", slog.String("path", WorkspacePath))
					return os.WriteFile(filepath.Join(WorkspacePath, ".envrc"), []byte("export TEST_ENV=hi"), 0444)
				}),
			},
			{
				Name:  "down",
				Usage: "Unmount the in-memory workspace",
				Action: withClient(func(ctx context.Context, _ *cli.Command, conn *systemd.Conn) error {
					done := make(chan string, 1)
					if _, err := conn.StopUnitContext(ctx, WorkspaceUnit, "replace", done); err != nil {
						return fmt.Errorf("could not unmount workspace: %w", err)
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case result := <-done:
						if result != "done" {
							return fmt.Errorf("%s did not stop successfully, finished with state: %s", WorkspaceUnit, result)
						}
					}
					slog.Info("workspace unmounted")
					return nil
				}),
			},
		},
	}
}

// withClient dials the system bus, runs fn, and always closes the connection.
func withClient(fn func(context.Context, *cli.Command, *systemd.Conn) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		c, err := systemd.NewSystemConnectionContext(ctx)
		if err != nil {
			return err
		}
		defer c.Close()
		return fn(ctx, cmd, c)
	}
}
