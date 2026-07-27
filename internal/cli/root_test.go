package cli

import (
	"io"

	"github.com/urfave/cli/v3"
)

// testRoot returns a root command whose usage output is discarded. A parse
// error (a mistyped flag in a test case) otherwise dumps the full help text
// over the assertion failure it caused.
func testRoot(sub *cli.Command) *cli.Command {
	return &cli.Command{
		Name:      "pki",
		Commands:  []*cli.Command{sub},
		Writer:    io.Discard,
		ErrWriter: io.Discard,
	}
}
