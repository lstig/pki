package cli

import (
	"context"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/lstig/pki/internal/dbus"
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
				Action: withClient(func(_ context.Context, _ *cli.Command, c *dbus.Client) error {
					if err := c.Start(WorkspaceUnit); err != nil {
						return err
					}
					slog.Info("workspace mounted", slog.String("path", WorkspacePath))
					return nil
				}),
			},
			{
				Name:  "down",
				Usage: "Unmount the in-memory workspace and free the RAM",
				Action: withClient(func(_ context.Context, _ *cli.Command, c *dbus.Client) error {
					if err := c.Stop(WorkspaceUnit); err != nil {
						return err
					}
					slog.Info("workspace unmounted")
					return nil
				}),
			},
		},
	}
}

// withClient dials the system bus, runs fn, and always closes the connection.
func withClient(fn func(context.Context, *cli.Command, *dbus.Client) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		c, err := dbus.Dial()
		if err != nil {
			return err
		}
		defer c.Close()
		return fn(ctx, cmd, c)
	}
}
