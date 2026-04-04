package cli

import (
	"context"
	"crypto/elliptic"
	"errors"
	"fmt"
	"iter"
	"log"
	"maps"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/urfave/cli/v3"
)

const (
	ECDSA             = "ecdsa"
	RSA               = "rsa"
	Ed25519           = "ed25519"
	DefaultKeyRequest = ECDSA
)

func newCAInitCmd() *cli.Command {
	var (
		defaultCurve = elliptic.P384().Params().Name
		curves       = map[string]int{
			elliptic.P256().Params().Name: elliptic.P256().Params().BitSize,
			elliptic.P384().Params().Name: elliptic.P384().Params().BitSize,
			elliptic.P521().Params().Name: elliptic.P521().Params().BitSize,
		}
		keys = map[string]*csr.KeyRequest{
			ECDSA:   {A: ECDSA, S: curves[defaultCurve]},
			RSA:     {A: RSA, S: 4096},
			Ed25519: {A: Ed25519},
		}
		req = &csr.CertificateRequest{
			KeyRequest: keys[DefaultKeyRequest],
			Names:      []csr.Name{{}},
			CA:         &csr.CAConfig{Expiry: (87660 * time.Hour).String()},
		}
	)

	var (
		caName     = &cli.StringFlag{Name: "common-name", Usage: "Certificate authority common name", Destination: &req.CN}
		caCountry  = &cli.StringFlag{Name: "country", Usage: "Two-letter country code", Destination: &req.Names[0].C}
		caState    = &cli.StringFlag{Name: "state", Usage: "State or province name", Destination: &req.Names[0].ST}
		caLocality = &cli.StringFlag{Name: "locality", Usage: "Locality (city) name", Destination: &req.Names[0].L}
		caOrg      = &cli.StringFlag{Name: "organization", Usage: "Organization name", Destination: &req.Names[0].O}
		caOrgUnit  = &cli.StringFlag{Name: "organization-unit", Usage: "Organization unit name", Destination: &req.Names[0].OU}

		algorithm = &cli.StringFlag{
			Name:  "algorithm",
			Usage: fmt.Sprintf("Cryptographic algorithm (choices: %s)", strings.Join(slices.Sorted(maps.Keys(keys)), ", ")),
			Value: req.KeyRequest.A,
			Action: func(_ context.Context, _ *cli.Command, s string) error {
				key, ok := keys[s]
				if !ok {
					return fmt.Errorf("unknown algorithm '%s'", s)
				}
				req.KeyRequest = key
				return nil
			},
		}
		ellipticCurve = &cli.StringFlag{
			Name:  "elliptic-curve",
			Usage: fmt.Sprintf("Elliptic curve (choices: %s)", strings.Join(slices.Sorted(maps.Keys(curves)), ", ")),
			Value: defaultCurve,
			Action: func(_ context.Context, _ *cli.Command, s string) error {
				size, ok := curves[s]
				if !ok {
					return fmt.Errorf("unknown elliptic curve '%s'", s)
				}
				keys[ECDSA].S = size
				return nil
			},
		}
		rsaKeySize = &cli.IntFlag{Name: "rsa-key-size", Usage: "RSA key size", Destination: &keys[RSA].S, Value: keys[RSA].S}
		expiration = &cli.StringFlag{Name: "expiration", Usage: "Certificate expiration time", Destination: &req.CA.Expiry, Value: req.CA.Expiry, Validator: func(s string) error {
			_, err := time.ParseDuration(s)
			return err
		}}

		forceFlag = &cli.BoolFlag{Name: "force", Usage: "Overwrite existing files", HideDefault: true}
		yesFlag   = &cli.BoolFlag{Name: "yes", Usage: "Automatically confirm request details", HideDefault: true}
	)

	cmd := &cli.Command{
		Name:  "init",
		Usage: "Generate certificate authority root certificate",
		Flags: []cli.Flag{
			caName,
			caCountry,
			caState,
			caLocality,
			caOrg,
			caOrgUnit,
			algorithm,
			ellipticCurve,
			rsaKeySize,
			expiration,
			forceFlag,
			yesFlag,
		},
		Action: func(ctx context.Context, _ *cli.Command) error {
			confirmed := yesFlag.IsSet()
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Enter a name for the certificate authority").
						Value(&req.CN).
						Validate(func(s string) error {
							if len(s) == 0 {
								return errors.New("please provide a name")
							}
							return nil
						}),
				).WithHide(caName.IsSet()),
				huh.NewGroup(
					huh.NewNote().
						Title("Subject Details").
						Description("_The following fields are optional_"),
					huh.NewInput().
						Title("Enter two-letter country code").
						Value(&req.Names[0].C),
					huh.NewInput().
						Title("Enter state or province name").
						Value(&req.Names[0].ST),
					huh.NewInput().
						Title("Enter locality (city) name").
						Value(&req.Names[0].L),
					huh.NewInput().
						Title("Enter organization name").
						Value(&req.Names[0].O),
					huh.NewInput().
						Title("Enter organization unit name").
						Value(&req.Names[0].OU),
				).WithHide(caCountry.IsSet() || caState.IsSet() || caLocality.IsSet() || caOrg.IsSet() || caOrgUnit.IsSet()),
				huh.NewGroup(
					huh.NewSelect[*csr.KeyRequest]().
						Title("Choose an algorithm").
						Options(slices.Collect(mapOptions(keys))...).
						Value(&req.KeyRequest),
				).WithHide(algorithm.IsSet()),
				huh.NewGroup(
					huh.NewSelect[int]().
						Title("Choose an RSA key size").
						Options(huh.NewOptions[int](2048, 4096, 6144, 8192)...).
						Value(&keys[RSA].S),
				).WithHideFunc(func() bool { return rsaKeySize.IsSet() || req.KeyRequest.A != RSA }),
				huh.NewGroup(
					huh.NewSelect[int]().
						Title("Choose an ECDSA curve").
						Options(slices.Collect(mapOptions(curves))...).
						Value(&keys[ECDSA].S),
				).WithHideFunc(func() bool { return ellipticCurve.IsSet() || req.KeyRequest.A != ECDSA }),
				huh.NewGroup(
					huh.NewInput().
						Title("Enter an expiration time").
						Validate(func(s string) error {
							_, err := time.ParseDuration(s)
							return err
						}).
						Value(&req.CA.Expiry),
				).WithHide(expiration.IsSet()),
				huh.NewGroup(
					huh.NewConfirm().
						Title("Does the request look correct?").
						DescriptionFunc(fmtRequstDetails(req), req).
						Value(&confirmed).
						WithHeight(7),
				).WithHide(yesFlag.IsSet()),
			)
			if err := form.RunWithContext(ctx); err != nil {
				return err
			}

			if !confirmed {
				return errors.New("user canceled")
			}

			var (
				err  error
				out  = map[string][]byte{}
				file = strings.NewReplacer(" ", "_", "-", "_", ".", "_").Replace(strings.ToLower(req.CN))
			)
			out[".pem"], out[".csr"], out["-key.pem"], err = initca.New(req)
			if err != nil {
				return err
			}
			for ext, data := range out {
				mode := os.FileMode(0644)
				if ext == "-key.pem" {
					mode = 0600
				}
				if err := writeFile(file+ext, mode, forceFlag.IsSet(), data); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

func mapOptions[Map ~map[string]T, T comparable](m Map) iter.Seq[huh.Option[T]] {
	return func(yield func(huh.Option[T]) bool) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if !yield(huh.NewOption(k, m[k])) {
				return
			}
		}
	}
}

func fmtRequstDetails(req *csr.CertificateRequest) func() string {
	return func() string {
		sb := &strings.Builder{}
		w := tabwriter.NewWriter(sb, 0, 0, 1, ' ', 0)
		fmt.Fprintf(w, "Name\t: %s\n", must(req.Name()))
		fmt.Fprintf(w, "Expires\t: ≈ %s\n", time.Now().Add(must(time.ParseDuration(req.CA.Expiry))).Format(time.RFC1123))
		fmt.Fprintf(w, "Algorithm\t: %s\n", req.KeyRequest.Algo())
		switch req.KeyRequest.Algo() {
		case "rsa":
			fmt.Fprintf(w, "Key Size\t: %d", req.KeyRequest.Size())
		case "ecdsa":
			fmt.Fprintf(w, "Curve\t: %d", req.KeyRequest.Size())
		}
		w.Flush()
		return sb.String()
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
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
