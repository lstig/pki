package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"

	"github.com/lstig/pki/internal/luks"
	"github.com/lstig/pki/internal/password"
	"github.com/lstig/pki/internal/workspace"
)

func newLUKSCmd() *cli.Command {
	return &cli.Command{
		Name:  "luks",
		Usage: "Manage LUKS-encrypted removable USB volumes",
		Commands: []*cli.Command{
			newLUKSListCmd(),
			newLUKSInitCmd(),
			newLUKSUnlockCmd(),
			newLUKSLockCmd(),
		},
	}
}

func newLUKSListCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List attached removable USB block devices",
		Action: withLUKS(func(ctx context.Context, cmd *cli.Command, client *luks.Client) error {
			devices, err := client.List(ctx)
			if err != nil {
				return err
			}
			// A diagnostic, not data: `pki luks list` piped into a filter
			// should yield nothing when there is nothing attached.
			if len(devices) == 0 {
				fmt.Fprintln(cmd.Root().ErrWriter, "no removable USB devices attached")
				return nil
			}
			w := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DEVICE\tSIZE\tMODEL\tCONTENT\tSTATUS")
			for _, d := range devices {
				device, model := d.Device, orDash(d.Model)
				if d.Partition {
					device = "└─" + strings.TrimPrefix(d.Device, "/dev/")
					model = ""
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", device, humanSize(d.Size), model, content(d), status(d))
			}
			return w.Flush()
		}),
	}
}

func newLUKSInitCmd() *cli.Command {
	var (
		yesFlag          = &cli.BoolFlag{Name: "yes", Usage: "Automatically confirm erasing the device", HideDefault: true}
		forceFlag        = &cli.BoolFlag{Name: "force", Usage: "Bypass the removable-USB safety check (DANGEROUS: can destroy the OS disk)", HideDefault: true}
		labelFlag        = &cli.StringFlag{Name: "label", Aliases: []string{"l"}, Usage: "Filesystem label (determines the mountpoint)", Value: "pki"}
		passwordFileFlag = &cli.StringFlag{Name: "password-file", Usage: "Write the volume password to `PATH` (mode 0600); required with --yes"}
	)

	return &cli.Command{
		Name:      "init",
		Usage:     "Format a device as a LUKS2-encrypted ext4 volume (DESTROYS all data)",
		ArgsUsage: "DEVICE",
		Flags:     []cli.Flag{yesFlag, forceFlag, labelFlag, passwordFileFlag},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Bool(yesFlag.Name) && cmd.String(passwordFileFlag.Name) == "" {
				return ctx, errors.New("--yes requires --password-file: without it the generated passphrase would be unrecoverable")
			}
			return ctx, nil
		},
		Action: withLUKS(func(ctx context.Context, cmd *cli.Command, client *luks.Client) error {
			device, err := deviceArg(cmd)
			if err != nil {
				return err
			}
			yes := cmd.Bool(yesFlag.Name)
			passphraseFile := cmd.String(passwordFileFlag.Name)
			if !yes {
				if err := confirmErase(ctx, device); err != nil {
					return err
				}
			}
			passphrase, err := initPassphrase(ctx, device, yes)
			if err != nil {
				return err
			}
			if passphraseFile != "" {
				if err := writePassphraseFile(passphraseFile, passphrase); err != nil {
					return err
				}
			}

			slog.Info("formatting device, this may take a while", slog.String("device", device))
			mountpoint, err := client.Init(ctx, device, passphrase, cmd.String(labelFlag.Name), cmd.Bool(forceFlag.Name))
			if err != nil {
				return err
			}
			slog.Info("device formatted and mounted", slog.String("device", device), slog.String("mountpoint", mountpoint))
			recordVolume(mountpoint)
			return nil
		}),
	}
}

func newLUKSUnlockCmd() *cli.Command {
	return &cli.Command{
		Name:      "unlock",
		Usage:     "Unlock a LUKS volume and mount its filesystem",
		ArgsUsage: "DEVICE",
		Action: withLUKS(func(ctx context.Context, cmd *cli.Command, client *luks.Client) error {
			device, err := deviceArg(cmd)
			if err != nil {
				return err
			}

			mountpoint, err := client.Unlock(ctx, device, passphrasePrompt(ctx, device))
			if err != nil {
				return err
			}
			slog.Info("volume unlocked", slog.String("device", device), slog.String("mountpoint", mountpoint))
			recordVolume(mountpoint)
			return nil
		}),
	}
}

func newLUKSLockCmd() *cli.Command {
	return &cli.Command{
		Name:      "lock",
		Usage:     "Unmount and lock a LUKS volume",
		ArgsUsage: "DEVICE",
		Action: withLUKS(func(ctx context.Context, cmd *cli.Command, client *luks.Client) error {
			device, err := deviceArg(cmd)
			if err != nil {
				return err
			}
			if err := client.Lock(ctx, device); err != nil {
				return err
			}
			slog.Info("volume locked", slog.String("device", device))
			forgetVolume()
			return nil
		}),
	}
}

// volumeEnv is the .envrc variable holding the mountpoint of the unlocked
// LUKS volume.
const volumeEnv = "PKI_VOLUME"

// confirmErase asks before destroying the device; declining is an error.
func confirmErase(ctx context.Context, device string) error {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Erase ALL data on %s?", device)).
				Description("The device will be reformatted as a LUKS2-encrypted ext4 volume.").
				Value(&confirmed),
		),
	)
	if err := form.RunWithContext(ctx); err != nil {
		return err
	}
	if !confirmed {
		return errors.New("user canceled")
	}
	return nil
}

// initPassphrase returns the passphrase for a new volume. A generated 15-word
// passphrase is the default; the form offers entering your own instead. With
// yesSet the form is skipped and the generated passphrase is returned.
func initPassphrase(ctx context.Context, device string, yesSet bool) (string, error) {
	p := &password.Passphrase{WordCount: 15, Delim: " "}
	generated, err := p.Generate()
	if err != nil {
		return "", err
	}
	if yesSet {
		return generated, nil
	}

	var (
		manual   string
		generate = true
		recorded = false
	)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[bool]().
				Title("Choose a passphrase source").
				Options(
					huh.NewOption("Generate a random passphrase (recommended)", true),
					huh.NewOption("Enter my own", false),
				).
				Value(&generate),
		),
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Enter a passphrase for %s", device)).
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if len(s) == 0 {
						return errors.New("please provide a passphrase")
					}
					return nil
				}).
				Value(&manual),
			huh.NewInput().
				Title("Confirm the passphrase").
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if s != manual {
						return errors.New("passphrases do not match")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return generate }),
		huh.NewGroup(
			huh.NewNote().
				Title("Generated passphrase").
				Description(generated+"\n\nWrite it down now — it cannot be recovered later."),
			huh.NewConfirm().
				Title("Have you recorded the passphrase?").
				Value(&recorded),
		).WithHideFunc(func() bool { return !generate }),
	)
	if err := form.RunWithContext(ctx); err != nil {
		return "", err
	}

	if !generate {
		return manual, nil
	}
	if !recorded {
		return "", errors.New("user canceled")
	}
	return generated, nil
}

// writePassphraseFile writes the passphrase to path with mode 0600. An
// existing regular file is never overwritten; non-regular paths such as
// /dev/fd/3 are written through.
func writePassphraseFile(path, passphrase string) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if fi, err := os.Lstat(path); err != nil {
		flags |= os.O_EXCL
	} else if fi.Mode().IsRegular() {
		return fmt.Errorf("%s already exists, refusing to overwrite it", path)
	}

	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("could not write passphrase file: %w", err)
	}
	if _, err := f.WriteString(passphrase + "\n"); err != nil {
		f.Close()
		return fmt.Errorf("could not write passphrase file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("could not write passphrase file: %w", err)
	}

	slog.Warn("passphrase written — record it somewhere durable, the volume cannot be recovered without it",
		slog.String("path", path))
	return nil
}

// passphrasePrompt returns a callback that asks for the device's passphrase.
func passphrasePrompt(ctx context.Context, device string) func() (string, error) {
	return func() (string, error) {
		var passphrase string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(fmt.Sprintf("Enter the passphrase for %s", device)).
					EchoMode(huh.EchoModePassword).
					Value(&passphrase),
			),
		)
		return passphrase, form.RunWithContext(ctx)
	}
}

// forgetVolume removes the mountpoint from the workspace .envrc. Failure is
// not fatal.
func forgetVolume() {
	dir, err := workspacePath()
	if err == nil {
		err = workspace.UnsetEnv(dir, volumeEnv)
	}
	if err != nil {
		slog.Warn("could not remove mountpoint from workspace .envrc", slog.Any("error", err))
	}
}

// recordVolume publishes the mountpoint to the workspace .envrc so scripts
// can reference $PKI_VOLUME. Failure is not fatal.
func recordVolume(mountpoint string) {
	dir, err := workspacePath()
	if err == nil {
		err = workspace.SetEnv(dir, volumeEnv, mountpoint)
	}
	if err != nil {
		slog.Warn("could not record mountpoint in workspace .envrc (is the workspace up?)", slog.Any("error", err))
		return
	}
	slog.Info("mountpoint recorded in workspace .envrc", slog.String("env", volumeEnv))
}

// withLUKS dials the system bus, runs fn, and always closes the connection.
func withLUKS(fn func(context.Context, *cli.Command, *luks.Client) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		client, err := luks.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()
		return fn(ctx, cmd, client)
	}
}

func deviceArg(cmd *cli.Command) (string, error) {
	device := cmd.Args().First()
	if device == "" {
		return "", errors.New("missing required argument: DEVICE")
	}
	return device, nil
}

func content(d luks.Device) string {
	s := d.Type
	if d.Label != "" {
		s += fmt.Sprintf(" (%s)", d.Label)
	}
	return orDash(s)
}

func status(d luks.Device) string {
	switch {
	case d.Encrypted && !d.Unlocked:
		return "locked"
	case d.Encrypted && len(d.MountPoints) == 0:
		return "unlocked"
	case d.Encrypted:
		return "unlocked, mounted on " + strings.Join(d.MountPoints, ", ")
	case len(d.MountPoints) > 0:
		return "mounted on " + strings.Join(d.MountPoints, ", ")
	default:
		return "-"
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func humanSize(n uint64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
