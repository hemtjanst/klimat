package status

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"

	"github.com/go-ocf/go-coap"
	"github.com/go-ocf/go-coap/codes"
	"github.com/peterbourgon/ff/v3/ffcli"
	"hemtjan.st/klimat/philips"
)

type config struct {
	out  io.Writer
	host string
}

// NewCmd returns the discover subcommand
func NewCmd(out io.Writer) *ffcli.Command {
	c := config{
		out: out,
	}

	fs := flag.NewFlagSet("klimat status", flag.ExitOnError)
	fs.StringVar(&c.host, "address", "localhost:5683", "host:port to connect to")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "status [flags]",
		FlagSet:    fs,
		ShortHelp:  "Status observes the machine state and dumps the messages",
		Exec:       c.Exec,
	}
}

func (c *config) Exec(ctx context.Context, args []string) error {
	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	obs, err := cl.Status(func(req *coap.Request) {
		if req.Msg.IsConfirmable() {
			m := req.Client.NewMessage(coap.MessageParams{
				Type:      coap.Acknowledgement,
				Code:      codes.Empty,
				MessageID: req.Msg.MessageID(),
			})
			m.SetOption(coap.ContentFormat, coap.TextPlain)
			m.SetOption(coap.LocationPath, req.Msg.Path())
			if err := req.Client.WriteMsg(m); err != nil {
				log.Printf("failed to acknowledge message: %v", err)
			}
		}

		resp, err := philips.DecodeMessage(req.Msg.Payload())
		if err != nil {
			log.Printf("failed to decode: %v, payload: %s", err, string(req.Msg.Payload()))
			return
		}

		var data philips.Status
		err = json.Unmarshal(resp, &data)
		if err != nil {
			log.Printf("failed to unmarshal JSON: %v", err)
			return
		}
		log.Printf("%++v", data.State.Reported)
	})

	if err != nil {
		return err
	}

	<-ctx.Done()
	obs.Cancel()

	return nil
}
