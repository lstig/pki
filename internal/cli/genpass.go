package cli

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

const base32alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func newGenpassCmd() *cli.Command {
	var (
		groups    = &cli.IntFlag{Name: "groups", Aliases: []string{"g"}, Usage: "Number of groups", Value: 8}
		groupSize = &cli.IntFlag{Name: "group-size", Aliases: []string{"s"}, Usage: "Size of each group", Value: 5}
		delimiter = &cli.StringFlag{Name: "delimiter", Aliases: []string{"d"}, Usage: "Separator between groups", Value: "-"}
	)

	cmd := &cli.Command{
		Name:  "genpass",
		Usage: "Generate a random password",
		Flags: []cli.Flag{
			groups,
			groupSize,
			delimiter,
		},
		Description: `Generate a random password consisting of groups of base32 characters separated by a delimiter.

The default settings generates a password with 40 characters which should provide at least 192 bits of randomness.

	log₃₂(2¹⁹²) = 39 characters

Examples:

# Generate a random password
$ pki genpass
2WV7N-QG36J-4TIDD-4XFZA-RLRWC-XJPUB-JWOH5-DQTAE

# Use a custom size
$ pki genpass --groups 4 --group-size 8
2BS6OHKJ-4AS5CCHZ-7BWA5MBP-XSGKRGNP

# Use a custom delimiter
$ pki genpass --delimiter=""
EALQEB644ZW6UEBLAMNLHFEBTM6GFTYTMCMXWDOW
		`,
		Action: func(_ context.Context, _ *cli.Command) error {
			length := groupSize.Value * groups.Value
			// go's rand.Text() implementation (which this function is based on) generates 26 characters by default,
			// which provides 128-bits of randomness. Weaker passwords may be vulnerable to bruteforce attacks.
			if length < 26 {
				fmt.Fprintf(os.Stderr, "WARNING: password may be weak, consider increasing the number of groups or group size\n")
			}
			src := make([]byte, length)
			rand.Read(src)
			var groups []string
			for chunk := range slices.Chunk(src, groupSize.Value) {
				sb := &strings.Builder{}
				for i := range chunk {
					sb.WriteByte(base32alphabet[chunk[i]%32])
				}
				groups = append(groups, sb.String())
			}
			fmt.Println(strings.Join(groups, delimiter.Value))
			return nil
		},
	}
	return cmd
}
