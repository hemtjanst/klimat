package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/go-ocf/go-coap"
	"github.com/go-ocf/go-coap/codes"
	"github.com/peterbourgon/ff/v3/ffcli"
	"hemtjan.st/klimat/philips"
)

type discoverConfig struct {
	out  io.Writer
	host string
}

func newDiscoverCmd(out io.Writer) *ffcli.Command {
	config := discoverConfig{
		out:  out,
		host: "",
	}

	discoverFlagset := flag.NewFlagSet("klimat discover", flag.ExitOnError)
	discoverFlagset.StringVar(&config.host, "address", "224.0.1.187:5683", "host:port for multicast discovery")

	return &ffcli.Command{
		Name:       "discover",
		ShortUsage: "discover [flags]",
		FlagSet:    discoverFlagset,
		ShortHelp:  "Discover compatible devices on the network",
		LongHelp: "The discover command uses multicat CoAP to discover devices " +
			"on the network. It implements the same discovery procedure as the " +
			"AirMatters app. The devices can be a bit finicky and may not always " +
			"respond, so you might have to run this a few times to ensure you get " +
			"a reply.",
		Exec: config.Exec,
	}
}

func (c *discoverConfig) Exec(ctx context.Context, args []string) error {
	client := &coap.MulticastClient{
		DialTimeout: 5 * time.Second,
	}

	conn, err := client.DialWithContext(ctx, c.host)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	req, err := conn.NewGetRequest("/sys/dev/info")
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	log.Print("sending discovery request")
	wait, err := conn.PublishMsgWithContext(ctx, req, func(req *coap.Request) {
		m := req.Client.NewMessage(coap.MessageParams{
			Type:      coap.Reset,
			Code:      codes.Empty,
			MessageID: req.Msg.MessageID(),
		})
		// I don't believe we should be sending a reset here, but it's what the
		// AirMatters app does according to packet captures, so lets do it
		if err := req.Client.WriteMsgWithContext(ctx, m); err != nil {
			log.Print("failed to send reset")
		}

		var info philips.Info
		if err := json.Unmarshal(req.Msg.Payload(), &info); err != nil {
			log.Printf("could not decode info: %v", err)
			return
		}
		log.Printf("discovered device at: %s: %+v", req.Client.RemoteAddr().String(), info)
	})
	if err != nil {
		return fmt.Errorf("failed to do discovery: %w", err)
	}

	// Wait for a couple of seconds to see if anyone responds
	for {
		select {
		case <-time.After(5 * time.Second):
			wait.Cancel()
			return nil
		case <-ctx.Done():
			wait.Cancel()
			return nil
		}
	}
}
