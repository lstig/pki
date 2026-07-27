package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// testRoot returns a root command wired like New does, with both streams
// captured. Tests read out for the command's data output and errOut for
// warnings and diagnostics.
func testRoot(sub *cli.Command) (root *cli.Command, out, errOut *bytes.Buffer) {
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	root = &cli.Command{
		Name:      "pki",
		Commands:  []*cli.Command{sub},
		Writer:    out,
		ErrWriter: errOut,
	}
	setUsageError(root)
	return root, out, errOut
}

// TestUsageErrorStaysOffStdout pins that a mistyped flag produces an error and
// writes nothing to the data stream: `pki genpass --bogus > pass.txt` must not
// fill the file with help text.
func TestUsageErrorStaysOffStdout(t *testing.T) {
	root, out, errOut := testRoot(newGenpassCmd())

	err := root.Run(context.Background(), []string{"pki", "genpass", "--bogus"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Errorf("error = %q, want the parse failure", err)
	}
	if !strings.Contains(err.Error(), "see 'pki genpass --help' for usage") {
		t.Errorf("error = %q, want a pointer to the subcommand help", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %q to stdout, want nothing", out)
	}
	if errOut.Len() != 0 {
		t.Errorf("wrote %q to stderr, want the error returned instead", errOut)
	}
}

// TestUsageErrorOnRoot pins the same for an unknown flag on the root command,
// which urfave handles on a separate path from subcommands.
func TestUsageErrorOnRoot(t *testing.T) {
	root, out, _ := testRoot(newGenpassCmd())

	if err := root.Run(context.Background(), []string{"pki", "--bogus"}); err == nil {
		t.Fatal("expected an error, got nil")
	}
	if out.Len() != 0 {
		t.Errorf("wrote %q to stdout, want nothing", out)
	}
}

// TestHelpGoesToStdout pins the other half of the rule: help that was asked
// for is the output, so it belongs on stdout.
func TestHelpGoesToStdout(t *testing.T) {
	root, out, errOut := testRoot(newGenpassCmd())

	if err := root.Run(context.Background(), []string{"pki", "genpass", "--help"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "--word-count") {
		t.Errorf("stdout = %q, want the genpass flags", out)
	}
	if errOut.Len() != 0 {
		t.Errorf("wrote %q to stderr, want help on stdout only", errOut)
	}
}
