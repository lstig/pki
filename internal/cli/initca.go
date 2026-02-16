package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/urfave/cli/v3"
)

func newInitCACommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "initca",
		Usage: "Generate certificate authority root certificate",
	}

	var (
		forceFlag = &cli.BoolFlag{Name: "force", Usage: "Overwrite existing files", HideDefault: true}
		yesFlag   = &cli.BoolFlag{Name: "yes", Usage: "Automatically confirm request details", HideDefault: true}
		fileFlag  = &cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "Override name prefix of the PEM, CSR, and Key"}
	)

	var (
		req       = csr.New()
		confirmed = true
		groups    []*huh.Group
	)
	groups = append(groups, configureSubject(cmd, req)...)
	groups = append(groups, configureCA(cmd, req)...)
	groups = append(groups, huh.NewGroup(
		huh.NewConfirm().
			Title("Does the request look correct?").
			DescriptionFunc(fmtRequstDetails(req), req).
			Value(&confirmed).
			WithHeight(7),
	).WithHideFunc(func() bool { return yesFlag.IsSet() }))

	cmd.Flags = append(cmd.Flags, yesFlag, forceFlag, fileFlag)
	cmd.Action = func(ctx context.Context, _ *cli.Command) error {
		if err := huh.NewForm(groups...).RunWithContext(ctx); err != nil {
			return err
		}

		if !confirmed {
			return errors.New("command aborted")
		}

		var (
			err error
			out = map[string][]byte{}
		)
		out[".pem"], out[".csr"], out["-key.pem"], err = initca.New(req)
		if err != nil {
			return err
		}

		if fileFlag.Value == "" {
			r := strings.NewReplacer(" ", "_", "-", "_", ".", "_")
			fileFlag.Value = r.Replace(strings.ToLower(req.CN))
		}

		for ext, data := range out {
			mode := os.FileMode(0644)
			if ext == "-key.pem" {
				mode = 0600
			}
			if err := writeFile(fileFlag.Value+ext, mode, forceFlag.IsSet(), data); err != nil {
				return err
			}
		}

		return nil
	}

	return cmd
}

func configureSubject(cmd *cli.Command, req *csr.CertificateRequest) []*huh.Group {
	req.Names = []csr.Name{{}}
	var (
		nameFlag     = &cli.StringFlag{Name: "common-name", Usage: "Certificate authority common name", Destination: &req.CN}
		countryFlag  = &cli.StringFlag{Name: "country", Usage: "Two-letter country code", Destination: &req.Names[0].C}
		stateFlag    = &cli.StringFlag{Name: "state", Usage: "State or province name", Destination: &req.Names[0].ST}
		localityFlag = &cli.StringFlag{Name: "locality", Usage: "Locality (city) name", Destination: &req.Names[0].L}
		orgFlag      = &cli.StringFlag{Name: "organization", Usage: "Organization name", Destination: &req.Names[0].O}
		orgUnitFlag  = &cli.StringFlag{Name: "organization-unit", Usage: "Organization unit name", Destination: &req.Names[0].OU}
	)
	cmd.Flags = append(cmd.Flags, nameFlag, countryFlag, stateFlag, localityFlag, orgFlag, orgUnitFlag)
	return []*huh.Group{
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
		).WithHideFunc(func() bool { return nameFlag.IsSet() }),
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
		).WithHideFunc(func() bool {
			return countryFlag.IsSet() || stateFlag.IsSet() || localityFlag.IsSet() || orgFlag.IsSet() || orgUnitFlag.IsSet()
		}),
	}
}

func configureCA(cmd *cli.Command, req *csr.CertificateRequest) []*huh.Group {
	var (
		ecdsa   = &csr.KeyRequest{A: "ecdsa", S: 384}
		rsa     = &csr.KeyRequest{A: "rsa", S: 4096}
		ed25519 = &csr.KeyRequest{A: "ed25519"}
	)
	req.KeyRequest = ecdsa
	req.CA = &csr.CAConfig{Expiry: (87660 * time.Hour).String()}
	var (
		algorithmFlag = &cli.StringFlag{
			Name:  "algorithm",
			Usage: fmt.Sprintf("Cryptographic algorithm: %s, %s, or %s", ecdsa.A, ed25519.A, rsa.A),
			Value: req.KeyRequest.A,
			Action: func(ctx context.Context, command *cli.Command, s string) error {
				switch s {
				case ecdsa.A:
					req.KeyRequest = ecdsa
				case rsa.A:
					req.KeyRequest = rsa
				case ed25519.A:
					req.KeyRequest = ed25519
				default:
					return fmt.Errorf("unsupported algorithm '%s'", s)
				}
				return nil
			},
		}
		ecdsaCurveFlag = &cli.IntFlag{Name: "ecdsa-curve", Usage: "ECDSA curve", Destination: &ecdsa.S, Value: ecdsa.S}
		rsaKeySizeFlag = &cli.IntFlag{Name: "rsa-key-size", Usage: "RSA key size", Destination: &rsa.S, Value: rsa.S}
		expirationFlag = &cli.StringFlag{Name: "expiration", Usage: "Certificate expiration time (in hours)", Destination: &req.CA.Expiry, Value: req.CA.Expiry, Validator: func(s string) error {
			_, err := time.ParseDuration(s)
			return err
		}}
	)
	cmd.Flags = append(cmd.Flags, algorithmFlag, ecdsaCurveFlag, rsaKeySizeFlag, expirationFlag)
	return []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[*csr.KeyRequest]().Title("Choose an algorithm").
				Options(
					huh.NewOption("RSA", rsa),
					huh.NewOption("ECDSA", ecdsa),
					huh.NewOption("Ed25519", ed25519),
				).
				Value(&req.KeyRequest),
		).WithHideFunc(func() bool { return algorithmFlag.IsSet() }),
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Choose an RSA key size").
				Options(huh.NewOptions[int](2048, 4096, 6144, 8192)...).
				Value(&rsa.S),
		).WithHideFunc(func() bool { return rsaKeySizeFlag.IsSet() || req.KeyRequest.A != rsa.A }),
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Choose an ECDSA curve").
				Options(huh.NewOptions[int](256, 384, 521)...).
				Value(&ecdsa.S),
		).WithHideFunc(func() bool { return ecdsaCurveFlag.IsSet() || req.KeyRequest.A != ecdsa.A }),
		huh.NewGroup(
			huh.NewInput().
				Title("Enter an expiration time").
				Validate(func(s string) error {
					_, err := time.ParseDuration(s)
					return err
				}).
				Value(&req.CA.Expiry),
		).WithHideFunc(func() bool { return expirationFlag.IsSet() }),
	}
}

func fmtRequstDetails(req *csr.CertificateRequest) func() string {
	return func() string {
		sb := &strings.Builder{}
		w := tabwriter.NewWriter(sb, 0, 0, 1, ' ', 0)
		fmt.Fprintf(w, "Name\t: %s\n", must(req.Name()))
		fmt.Fprintf(w, "Expires\t: %s\n", time.Now().Add(must(time.ParseDuration(req.CA.Expiry))).Format(time.RFC1123))
		fmt.Fprintf(w, "Algorithm\t: %s\n", req.KeyRequest.Algo())
		switch req.KeyRequest.Algo() {
		case "rsa":
			fmt.Fprintf(w, "Key Size\t: %d", req.KeyRequest.Size())
		case "ecdsa":
			fmt.Fprintf(w, "ECDSA Curve\t: %d", req.KeyRequest.Size())
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
