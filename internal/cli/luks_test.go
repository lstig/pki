package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// TestLUKSInitFlagResolution pins that `luks init` reads parsed flag values:
// an explicit --yes=false must still confirm and --label must be honoured.
func TestLUKSInitFlagResolution(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantYes   bool
		wantForce bool
		wantLabel string
	}{
		{"defaults", nil, false, false, "pki"},
		{"yes", []string{"--yes", "--password-file", "pass.txt"}, true, false, "pki"},
		{"explicit yes=false", []string{"--yes=false"}, false, false, "pki"},
		{"force", []string{"--force"}, false, true, "pki"},
		{"explicit force=false", []string{"--force=false"}, false, false, "pki"},
		{"label", []string{"--label", "vault"}, false, false, "vault"},
		{"label shorthand", []string{"-l", "vault"}, false, false, "vault"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotYes, gotForce bool
				gotLabel         string
			)

			init := newLUKSInitCmd()
			yesFlag, forceFlag, labelFlag := init.Flags[0], init.Flags[1], init.Flags[2]
			// replace the destructive action with the same flag resolution
			init.Action = func(_ context.Context, cmd *cli.Command) error {
				gotYes = cmd.Bool(yesFlag.Names()[0])
				gotForce = cmd.Bool(forceFlag.Names()[0])
				gotLabel = cmd.String(labelFlag.Names()[0])
				return nil
			}

			root := testRoot(init)
			args := append([]string{"pki", "init"}, tt.args...)
			if err := root.Run(context.Background(), append(args, "/dev/sdb")); err != nil {
				t.Fatalf("run: %v", err)
			}

			if gotYes != tt.wantYes {
				t.Errorf("yes = %v, want %v", gotYes, tt.wantYes)
			}
			if gotForce != tt.wantForce {
				t.Errorf("force = %v, want %v", gotForce, tt.wantForce)
			}
			if gotLabel != tt.wantLabel {
				t.Errorf("label = %q, want %q", gotLabel, tt.wantLabel)
			}
		})
	}
}

// TestInitPassphraseNonInteractive pins that --yes never builds a form. huh
// short-circuits only on group count, not on group visibility, so a form built
// with every group hidden still opens /dev/tty and fails under systemd or CI.
func TestInitPassphraseNonInteractive(t *testing.T) {
	got, err := initPassphrase(context.Background(), "/dev/sdb", true)
	if err != nil {
		t.Fatalf("initPassphrase: %v", err)
	}
	if words := len(strings.Fields(got)); words != 15 {
		t.Errorf("generated %d words, want 15: %q", words, got)
	}
}

// TestLUKSInitRequiresPassphraseFile pins that --yes without --password-file
// is refused before any D-Bus connection is opened.
func TestLUKSInitRequiresPassphraseFile(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"yes without password-file", []string{"--yes"}, true},
		{"yes with password-file", []string{"--yes", "--password-file", "pass.txt"}, false},
		{"interactive without password-file", nil, false},
		{"explicit yes=false", []string{"--yes=false"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := newLUKSInitCmd()
			init.Action = func(context.Context, *cli.Command) error { return nil }

			root := testRoot(init)
			args := append([]string{"pki", "init"}, tt.args...)
			err := root.Run(context.Background(), append(args, "/dev/sdb"))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if !strings.Contains(err.Error(), "--password-file") {
					t.Errorf("error %q does not mention --password-file", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}

// TestWritePassphraseFile pins that the file is created 0600 and that an
// existing regular file is never clobbered.
func TestWritePassphraseFile(t *testing.T) {
	const pass = "correct horse battery staple"

	t.Run("creates 0600 with trailing newline", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "pass.txt")
		if err := writePassphraseFile(path, pass); err != nil {
			t.Fatalf("writePassphraseFile: %v", err)
		}

		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if got := fi.Mode().Perm(); got != 0o600 {
			t.Errorf("mode = %v, want -rw-------", got)
		}

		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(b) != pass+"\n" {
			t.Errorf("content = %q, want %q", b, pass+"\n")
		}
	})

	t.Run("refuses to overwrite an existing regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "pass.txt")
		if err := os.WriteFile(path, []byte("previous volume\n"), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}

		if err := writePassphraseFile(path, pass); err == nil {
			t.Fatal("expected an error, got nil")
		}

		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(b) != "previous volume\n" {
			t.Errorf("existing file was modified: %q", b)
		}
	})

	t.Run("writes through a non-regular file", func(t *testing.T) {
		if err := writePassphraseFile(os.DevNull, pass); err != nil {
			t.Errorf("writePassphraseFile(%s): %v", os.DevNull, err)
		}
	})
}
