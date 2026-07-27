package cli

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// runGenpass runs `pki genpass` with args and returns what the command printed.
// The action writes to os.Stdout directly, so the pipe swap is the only way to
// observe which generator the flags were wired to.
func runGenpass(t *testing.T, args ...string) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w
	root := testRoot(newGenpassCmd())
	runErr := root.Run(context.Background(), append([]string{"pki", "genpass"}, args...))
	os.Stdout = orig

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}

	return strings.TrimSuffix(string(out), "\n"), runErr
}

// TestGenpassFlags pins that each flag reaches the generator it belongs to:
// --mode picks the generator, and the size flags are wired to that generator's
// destination fields.
func TestGenpassFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantDelim string
		wantParts int
		wantSize  int // characters per part, 0 to skip (word lengths vary)
	}{
		{"defaults", nil, " ", 12, 0},
		{"word count", []string{"--word-count", "10"}, " ", 10, 0},
		{"word count shorthand", []string{"-n", "10"}, " ", 10, 0},
		{"explicit passphrase mode", []string{"--mode", "passphrase"}, " ", 12, 0},
		{"base32 mode", []string{"--mode", "base32"}, "-", 6, 5},
		{"base32 mode shorthand", []string{"-m", "base32"}, "-", 6, 5},
		{"base32 size", []string{"-m", "base32", "--group-count", "4", "--group-size", "8"}, "-", 4, 8},
		{"base32 size shorthand", []string{"-m", "base32", "-g", "4", "-s", "8"}, "-", 4, 8},
		// The size flags belong to the other mode's generator and must not
		// change the selected one.
		{"word count ignored in base32 mode", []string{"-m", "base32", "-n", "3"}, "-", 6, 5},
		{"group flags ignored in passphrase mode", []string{"-g", "1", "-s", "1"}, " ", 12, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runGenpass(t, tt.args...)
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			parts := strings.Split(out, tt.wantDelim)
			if len(parts) != tt.wantParts {
				t.Fatalf("got %d parts %q, want %d", len(parts), parts, tt.wantParts)
			}
			for _, p := range parts {
				if p == "" {
					t.Fatalf("empty part in %q", out)
				}
				if tt.wantSize > 0 && len(p) != tt.wantSize {
					t.Errorf("part %q has size %d, want %d", p, len(p), tt.wantSize)
				}
			}
		})
	}
}

// TestGenpassInvalidFlags pins that bad values are refused rather than
// producing a weak password.
func TestGenpassInvalidFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"unknown mode", []string{"--mode", "rot13"}, "invalid mode: rot13"},
		{"empty mode", []string{"--mode", ""}, "invalid mode"},
		{"zero word count", []string{"--word-count", "0"}, "out of range"},
		{"negative word count", []string{"--word-count", "-1"}, "out of range"},
		{"word count above wordlist size", []string{"--word-count", "7777"}, "out of range"},
		{"zero group count", []string{"-m", "base32", "--group-count", "0"}, "group count must be greater than zero"},
		{"zero group size", []string{"-m", "base32", "--group-size", "0"}, "group size must be greater than zero"},
		{"unknown flag", []string{"--rounds", "4"}, "flag provided but not defined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runGenpass(t, tt.args...)
			if err == nil {
				t.Fatalf("expected an error, got nil (printed %q)", out)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
			if out != "" {
				t.Errorf("printed %q, want nothing", out)
			}
		})
	}
}

// TestGenpassDefaultMode pins the default mode against the flag validator,
// which runs on the default value too.
func TestGenpassDefaultMode(t *testing.T) {
	cmd := newGenpassCmd()
	mode := cmd.Flags[0]
	if got := mode.Names()[0]; got != "mode" {
		t.Fatalf("first flag = %q, want mode", got)
	}
	if got := mode.(*cli.StringFlag).Value; got != "passphrase" {
		t.Errorf("default mode = %q, want passphrase", got)
	}
}
