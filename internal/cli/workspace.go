package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v3"

	"github.com/lstig/pki/internal/systemd"
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
				Action: withClient(func(ctx context.Context, _ *cli.Command, conn *dbus.Conn) error {
					if err := systemd.StartUnit(ctx, conn, WorkspaceUnit); err != nil {
						return fmt.Errorf("could not mount workspace: %w", err)
					}
					slog.Info("workspace mounted", slog.String("path", WorkspacePath))
					return nil
				}),
			},
			{
				Name:  "down",
				Usage: "Unmount the in-memory workspace",
				Action: withClient(func(ctx context.Context, _ *cli.Command, conn *dbus.Conn) error {
					if err := systemd.StopUnit(ctx, conn, WorkspaceUnit); err != nil {
						return fmt.Errorf("could not unmount workspace: %w", err)
					}
					slog.Info("workspace unmounted")
					return nil
				}),
			},
		},
	}
}

// withClient dials the system bus, runs fn, and always closes the connection.
func withClient(fn func(context.Context, *cli.Command, *dbus.Conn) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		c, err := dbus.ConnectSystemBus()
		if err != nil {
			return err
		}
		defer c.Close()
		return fn(ctx, cmd, c)
	}
}
