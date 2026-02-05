package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/urfave/cli/v3"
)

func newInitCACommand() *cli.Command {
	var (
		filename  = &cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Override name prefix of the PEM, CSR, and Key"}
		force     = &cli.BoolFlag{Name: "force", Usage: "Overwrite existing files", HideDefault: true}
		algorithm = &cli.StringFlag{
			Name:    "algorithm",
			Aliases: []string{"a"},
			Usage:   "Cryptographic algorithm: rsa, ecdsa, or ed25519",
			Value:   "rsa",
			Validator: func(s string) error {
				switch s {
				case "rsa", "ecdsa", "ed25519":
					return nil
				}
				return fmt.Errorf("invalid algorithm '%s'", s)
			},
		}
		size             = &cli.IntFlag{Name: "size", Aliases: []string{"s"}, Usage: "RSA key size or ECDSA curve", Value: 4096}
		expiry           = &cli.DurationFlag{Name: "expiry", Aliases: []string{"e"}, Usage: "Certificate expiration time", Value: 87660 * time.Hour}
		country          = &cli.StringFlag{Name: "country", Usage: "Two-letter country code"}
		state            = &cli.StringFlag{Name: "state", Usage: "State or province name"}
		locality         = &cli.StringFlag{Name: "locality", Usage: "Locality (city) name"}
		organization     = &cli.StringFlag{Name: "organization", Usage: "Organization name"}
		organizationUnit = &cli.StringFlag{Name: "organization-unit", Usage: "Organization unit name"}
	)

	cmd := &cli.Command{
		Name:  "initca",
		Usage: "Generate certificate authority root certificate",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "<common name>"},
		},
		Flags: []cli.Flag{
			filename,
			force,
			algorithm,
			size,
			expiry,
			country,
			state,
			locality,
			organization,
			organizationUnit,
		},
		Before: func(ctx context.Context, command *cli.Command) (context.Context, error) {
			if command.Args().Len() != 1 {
				return ctx, fmt.Errorf("expected exactly one argument but found %d", command.Args().Len())
			}
			return ctx, nil
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			req := csr.CertificateRequest{
				CN: command.StringArg("<common name>"),
				CA: &csr.CAConfig{
					Expiry: expiry.Value.String(),
				},
				Names: []csr.Name{
					{
						C:  country.Value,
						ST: state.Value,
						L:  locality.Value,
						O:  organization.Value,
						OU: organizationUnit.Value,
					},
				},
				KeyRequest: csr.NewKeyRequest(),
			}

			switch algorithm.Value {
			case "rsa", "ecdsa":
				req.KeyRequest.A = algorithm.Value
				req.KeyRequest.S = size.Value
			case "ed25519":
				req.KeyRequest.A = algorithm.Value
			}

			var (
				err error
				out = map[string][]byte{}
			)
			out[".pem"], out[".csr"], out["-key.pem"], err = initca.New(&req)
			if err != nil {
				return err
			}

			if filename.Value == "" {
				sr := strings.NewReplacer(" ", "_", "-", "_", ".", "_")
				filename.Value = sr.Replace(strings.ToLower(req.CN))
			}

			for ext, data := range out {
				mode := os.FileMode(0644)
				if ext == "-key.pem" {
					mode = 0600
				}
				if err := writeFile(filename.Value+ext, mode, force.IsSet(), data); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

func writeFile(file string, mode os.FileMode, overwrite bool, data []byte) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(file, flags, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}
