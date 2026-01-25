package main

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

type initCAOptions struct {
	filename         string
	force            bool
	algo             string
	size             int
	expiry           time.Duration
	country          string
	state            string
	locality         string
	organization     string
	organizationUnit string
}

func newInitCACommand() *cli.Command {
	var (
		filename = &cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Override name of PEM, CSR, and Key"}
		//force            = &cli.BoolFlag{Name: "force", Usage: "Overwrite existing files"}
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
			algorithm,
			size,
			expiry,
			country,
			state,
			locality,
			organization,
			organizationUnit,
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
				perm := os.FileMode(0644)
				if ext == "-key.pem" {
					perm = 0600
				}
				if err := os.WriteFile(filename.Value+ext, data, perm); err != nil {
					return err
				}
			}

			return nil
		},
	}

	//"$PROG $CMD [options] NAME"                                                                                           \
	//""                                                                                                                    \
	//"  Options:"                                                                                                          \
	//"    -f,  --filename string            Override name of PEM, CSR, and Key"                                            \
	//"    --force                           Overwrite existing files"                                                      \
	//"    -a,  --algorithm string                Cryptographic algorithm: rsa, ecdsa, or ed25519 (default \"${key_algo[-1]}\")" \
	//"    -s,  --size int                   RSA key size or ECDSA curve (default ${key_size[-1]})"                         \
	//"    -e,  --expiry string              The validity period of the certificate (default \"${expiry[-1]}\")"            \
	//"    -C,  --country string             Two-letter country code"                                                       \
	//"    -ST, --state string               State or province name"                                                        \
	//"    -L,  --locality string            Locality (city) name"                                                          \
	//"    -O,  --organization string        Organization name"                                                             \
	//"    -OU, --organization-unit string   Organization unit name"                                                        \

	return cmd
}
