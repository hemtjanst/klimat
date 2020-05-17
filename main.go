package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/go-ocf/go-coap"
	"github.com/go-ocf/go-coap/codes"

	"hemtjan.st/klimat/philips"
	"lib.hemtjan.st/client"
	"lib.hemtjan.st/device"
	"lib.hemtjan.st/feature"
	"lib.hemtjan.st/transport/mqtt"
)

const (
	twoWeeks = 336 // hours
)

var (
	hostPort = flag.String("address", "127.0.0.1:5683", "host:port for the purifier")
	debug    = flag.Bool("debug", false, "Debug, prints lots of the raw payloads")
)

func connectToDevice(ctx context.Context, address string) *coap.ClientConn {
	cl := coap.Client{
		Net:         "udp",
		DialTimeout: 5 * time.Second,
		// Internally the time is divided by 6, so this results in a ping/pong every 5s
		// which is what the Air Matters app does
		KeepAlive: coap.MustMakeKeepAlive(30 * time.Second),
	}

	conn, err := cl.DialWithContext(ctx, address)
	if err != nil {
		log.Fatalf("Error dialing: %v", err)
	}
	return conn
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

func getWithTimeout(ctx context.Context, cl *coap.ClientConn, path string) (coap.Message, error) {
	timeout, tcancel := context.WithTimeout(ctx, 5*time.Second)
	defer tcancel()
	return cl.GetWithContext(timeout, path)
}

func run(ctx context.Context, address string, mqttConfig *mqtt.Config, out io.Writer) error {
	log.SetOutput(out)

	cl := connectToDevice(ctx, address)
	devInfo, err := getWithTimeout(ctx, cl, "/sys/dev/info")
	if err != nil {
		return fmt.Errorf("could not get device info: %w", err)
	}
	log.Print("Received device info")
	if *debug {
		log.Printf("raw info: %s", devInfo.Payload())
	}

	var info philips.Info
	if err := json.Unmarshal(devInfo.Payload(), &info); err != nil {
		return fmt.Errorf("could not decode info: %w", err)
	}

	if *debug {
		log.Printf("info: %+v", info)
	}

	mq := connectMqtt(ctx, mqttConfig)
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

	sess := philips.NewID()
	rsp, err := cl.PostWithContext(ctx, "/sys/dev/sync", coap.TextPlain, bytes.NewReader([]byte(sess.Hex())))
	if err != nil {
		return fmt.Errorf("failed to post to /sys/dev/sync and get session: %w", err)
	}

	myId := philips.ParseID(rsp.Payload()) + 1
	idLock := sync.RWMutex{}

	obs, err := cl.ObserveWithContext(ctx, "/sys/dev/status", handleObserve(dev))
	if err != nil {
		return fmt.Errorf("failed to start observe on /sys/dev/status: %w", err)
	}

	log.Print("Done initialising, publishing updates to MQTT")

	_ = dev.Feature("on").OnSetFunc(func(v string) {
		if v != "1" {
			v = "0"
		}
		data := `{"state":{"desired":{"pwr":"` + v + `"}}}`
		idLock.Lock()
		defer idLock.Unlock()
		newMsg, err := philips.EncodeMessage(myId, []byte(data))
		if err != nil {
			return
		}
		myId++
		_, _ = cl.Post("/sys/dev/control", coap.AppJSON, bytes.NewReader(newMsg))
	})

	<-ctx.Done()
	obs.Cancel()

	return nil
}

func main() {
	mqCfg := mqtt.MustFlags(flag.String, flag.Bool)
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

	err := run(ctx, *hostPort, mqCfg(), os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}

func handleObserve(dev client.Device) func(req *coap.Request) {
	// If the message was confirmable, confirm it before
	// proceeding with decoding it. This ensures that even
	// if we hit decoding issues, we always confirm the
	// message so the device continues sending new messages
	return func(req *coap.Request) {
		if *debug {
			log.Printf("payload: %s", req.Msg.Payload())
		}

		if req.Msg.IsConfirmable() {
			m := req.Client.NewMessage(coap.MessageParams{
				Type:      coap.Acknowledgement,
				Code:      codes.Empty,
				MessageID: req.Msg.MessageID(),
			})
			m.SetOption(coap.ContentFormat, coap.TextPlain)
			m.SetOption(coap.LocationPath, req.Msg.Path())
			if err := req.Client.WriteMsg(m); err != nil {
				log.Print("failed to acknowledge message")
			}
		}

		resp, err := philips.DecodeMessage(req.Msg.Payload())
		if err != nil {
			log.Println(err)
			return
		}
		if *debug {
			log.Printf("decoded message: %s", resp)
		}

		var data philips.Status
		err = json.Unmarshal(resp, &data)
		if err != nil {
			log.Println(err)
			return
		}
		if *debug {
			log.Printf("status: %+v", data)
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
