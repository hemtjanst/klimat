package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/go-ocf/go-coap"
	"github.com/go-ocf/go-coap/codes"
	"github.com/peterbourgon/ff/v3/ffcli"
	"hemtjan.st/klimat/philips"
	"lib.hemtjan.st/client"
	"lib.hemtjan.st/device"
	"lib.hemtjan.st/feature"
	"lib.hemtjan.st/transport/mqtt"
)

const (
	twoWeeks = 336 // hours
)

type publishConfig struct {
	out     io.Writer
	host    string
	mqttcfg func() *mqtt.Config
	debug   bool
}

func newPublishCmd(out io.Writer) *ffcli.Command {
	publishFlagset := flag.NewFlagSet("klimat publish", flag.ExitOnError)
	mqCfg := mqtt.MustFlags(publishFlagset.String, publishFlagset.Bool)

	config := publishConfig{
		out:     out,
		host:    "",
		mqttcfg: mqCfg,
	}

	publishFlagset.StringVar(&config.host, "address", "localhost:5683", "host:port to connect to")
	publishFlagset.BoolVar(&config.debug, "debug", false, "enable debug output")

	return &ffcli.Command{
		Name:       "publish",
		ShortUsage: "publish [flags]",
		ShortHelp:  "Publish sensor data to MQTT",
		LongHelp: "The publish command connects to a device over CoAP and " +
			"starts to observe it. As it receives updates the device state and " +
			"sensor data is extracted and published to MQTT.",
		FlagSet: publishFlagset,
		Exec:    config.Exec,
	}
}

func (c *publishConfig) Exec(ctx context.Context, args []string) error {
	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	info, err := cl.Info()
	if err != nil {
		return err
	}

	cfg := c.mqttcfg()
	mq := connectMqtt(ctx, cfg)
	dev, err := client.NewDevice(&device.Info{
		Topic:        fmt.Sprintf("climate/%s", info.DeviceID),
		Name:         info.Name,
		Manufacturer: "Philips",
		Model:        info.ModelID,
		SerialNumber: info.DeviceID,
		Type:         "airPurifier",
		Features: map[string]*feature.Info{
			"on":                                 {},
			"brightness":                         {},
			"currentAirPurifierState":            {},
			"targetAirPurifierState":             {},
			"currentFanState":                    {},
			"targetFanState":                     {},
			"rotationSpeed":                      {},
			"lockPhysicalControls":               {},
			"airQuality":                         {},
			"pm2_5Density":                       {},
			"filterChangeIndication":             {},
			"currentRelativeHumidity":            {},
			"targetRelativeHumidity":             {},
			"currentHumidifierDehumidifierState": {},
			"targetHumidifierDehumidifierState":  {},
			"currentTemperature":                 {},
			"waterLevel":                         {},
		},
	}, mq)
	if err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}

	_, err = cl.Session()
	if err != nil {
		return fmt.Errorf("failed to initialise new session: %w", err)
	}

	log.Print("starting observer for status messages")
	obs, err := cl.Status(handleObserve(dev))
	if err != nil {
		return err
	}

	log.Printf("Done initialising, publishing updates to MQTT on: %s", cfg.Address)

	<-ctx.Done()
	obs.Cancel()

	return nil
}

func connectMqtt(ctx context.Context, config *mqtt.Config) mqtt.MQTT {
	tr, err := mqtt.New(ctx, config)
	if err != nil {
		log.Fatalf("Error creating MQTT client: %v", err)
	}

	go func() {
		for {
			ok, err := tr.Start()
			if !ok {
				break
			}
			log.Printf("Error, retrying in 5 seconds: %v", err)
			time.Sleep(5 * time.Second)
		}
		os.Exit(1)
	}()

	return tr
}

func handleObserve(dev client.Device) func(req *coap.Request) {
	// If the message was confirmable, confirm it before
	// proceeding with decoding it. This ensures that even
	// if we hit decoding issues, we always confirm the
	// message so the device continues sending new messages
	return func(req *coap.Request) {
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

		update := data.State.Reported

		dev.Feature("on").Update(update.Power.ToHemtjanst())
		// Possible states are 0, 1 and 2, but since this device is only a humidifier
		// it can only ever be 1
		dev.Feature("targetHumidifierDehumidifierState").Update("1")
		if update.ChildLock {
			dev.Feature("lockPhysicalControls").Update("1")
		} else {
			dev.Feature("lockPhysicalControls").Update("0")
		}

		if update.Mode == philips.Manual {
			dev.Feature("targetAirPurifierState").Update("0")
			dev.Feature("targetFanState").Update("0")
		} else {
			dev.Feature("targetAirPurifierState").Update("1")
			dev.Feature("targetFanState").Update("1")
		}

		if update.Power == philips.On {
			// Only update certain values, like the sensors and operating aspects
			// if the device is on
			dev.Feature("brightness").Update(update.Brightness.ToHemtjanst())
			dev.Feature("currentAirPurifierState").Update("2")
			dev.Feature("currentFanState").Update("2")
			dev.Feature("rotationSpeed").Update(update.FanSpeed.ToHemtjanst())
			dev.Feature("airQuality").Update(update.AirQuality.ToHemtjanst())
			dev.Feature("pm2_5Density").Update(strconv.Itoa(int(math.Min(float64(update.ParticulateMatter25), 100))))
			// HomeKit doesn't really have the concept of multiple filters, each of which
			// could need changing, so flip this value if any of the filters need changing
			// or cleaning
			if update.ActiveCarbonFilterReplaceIn <= twoWeeks ||
				update.HEPAFilterReplaceIn <= twoWeeks ||
				update.WickReplaceIn <= twoWeeks ||
				update.PrefilterAndWickCleanIn <= 0 ||
				update.Err == philips.ErrCleanFilter {
				dev.Feature("filterChangeIndication").Update("1")
			} else {
				dev.Feature("filterChangeIndication").Update("0")
			}
			dev.Feature("currentRelativeHumidity").Update(strconv.Itoa(update.RelativeHumidity))
			dev.Feature("targetRelativeHumidity").Update(strconv.Itoa(update.RelativeHumidityTarget))
			dev.Feature("currentHumidifierDehumidifierState").Update(update.Function.ToHemtjanst())
			dev.Feature("currentTemperature").Update(strconv.Itoa(update.Temperature))
			dev.Feature("waterLevel").Update(strconv.Itoa(update.WaterLevel))
		} else {
			// Set certain values to 0 when we turn the device off so it looks like
			// it's not doing anything
			dev.Feature("brightness").Update("0")
			dev.Feature("currentAirPurifierState").Update("0")
			dev.Feature("currentFanState").Update("0")
			dev.Feature("rotationSpeed").Update("0")
			dev.Feature("currentHumidifierDehumidifierState").Update("0")
		}
	}
}
