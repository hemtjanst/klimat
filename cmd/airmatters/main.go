package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/go-ocf/go-coap"
	"github.com/go-ocf/go-coap/codes"
	"hemtjan.st/klimat/philips"
)

var (
	hostPort = flag.String("address", "224.0.1.187:5683", "host:port for multicast discovery")
)

func main() {
	flag.Parse()

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

	if err := run(ctx, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out io.Writer) error {
	log.SetOutput(out)

	client := &coap.MulticastClient{
		DialTimeout: 5 * time.Second,
	}

	conn, err := client.DialWithContext(ctx, *hostPort)
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
