package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/peterbourgon/ff/v3/ffcli"

	"hemtjan.st/klimat/cmd/klimat/control"
	"hemtjan.st/klimat/cmd/klimat/discover"
	"hemtjan.st/klimat/cmd/klimat/publish"
	"hemtjan.st/klimat/cmd/klimat/status"
)

var (
	rootFlagset = flag.NewFlagSet("klimat", flag.ExitOnError)

	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	log.SetOutput(os.Stdout)

	var fversion bool
	rootFlagset.BoolVar(&fversion, "version", false, "print version info")

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
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
		FlagSet: rootFlagset,
		Subcommands: []*ffcli.Command{
			control.NewCmd(os.Stdout),
			discover.NewCmd(os.Stdout),
			publish.NewCmd(os.Stdout),
			status.NewCmd(os.Stdout),
		},
		Exec: func(context.Context, []string) error {
			if fversion {
				fmt.Fprintf(os.Stdout, `{"version": "%s", "commit": "%s", "date": "%s"}`, version, commit, date)
				os.Exit(0)
			}
			return flag.ErrHelp
		},
	}

	if err := root.ParseAndRun(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
