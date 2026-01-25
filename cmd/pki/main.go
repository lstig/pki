package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/signal"

	"github.com/urfave/cli/v3"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	cmd := &cli.Command{
		Name:        "pki",
		Usage:       "Air-gapped PKI helper utility",
		HideVersion: true,
		Commands: []*cli.Command{
			newGenpassCmd(),
			newInitCACommand(),
			newVersionCmd(),
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
