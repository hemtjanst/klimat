package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/peterbourgon/ff/v3/ffcli"

	"hemtjan.st/klimat/cmd/klimat/discover"
	"hemtjan.st/klimat/cmd/klimat/publish"
)

var (
	rootFlagset = flag.NewFlagSet("klimat", flag.ExitOnError)
)

func main() {
	log.SetOutput(os.Stdout)

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			log.Print("Received cancellation signal, shutting down...")
			cancel()
		case <-ctx.Done():
		}
	}()

	root := &ffcli.Command{
		ShortUsage: "klimat [flags] <subcommand>",
		LongHelp: "This CLI can be used to interact with climate devices. " +
			"Right now it only supports interafcing with Philips AirCombi " +
			"devices.",
		FlagSet:     rootFlagset,
		Subcommands: []*ffcli.Command{discover.NewCmd(os.Stdout), publish.NewCmd(os.Stdout)},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}

	if err := root.ParseAndRun(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
