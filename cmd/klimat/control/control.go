package control

import (
	"context"
	"flag"
	"io"
	"log"
	"strconv"
	"strings"

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

	fs := flag.NewFlagSet("klimat control", flag.ExitOnError)
	fs.StringVar(&c.host, "address", "localhost:5683", "host:port to connect to")

	subcommands := []*ffcli.Command{
		{
			Name:       "brightness",
			ShortUsage: "brightness on|off|25|50|75",
			Exec:       c.brightness,
		},
		{
			Name:       "display",
			ShortUsage: "display humidity|iaq|pm25",
			Exec:       c.display,
		},
		{
			Name:       "fan",
			ShortUsage: "fan silent|1|2|3|turbo",
			Exec:       c.fanspeed,
		},
		{
			Name:       "function",
			ShortUsage: "function humidification|purification",
			LongHelp: "The humidification mode implies purification, whereas " +
				"purification does not imply humidification",
			Exec: c.function,
		},
		{
			Name:       "humidity",
			ShortUsage: "humidity 40|50|60|max",
			Exec:       c.humidity,
		},
		{
			Name:       "lock",
			ShortUsage: "lock on|yes|off|no",
			Exec:       c.lock,
		},
		{
			Name:       "mode",
			ShortUsage: "mode auto|allergen|bacteria|manual|night|sleep",
			LongHelp:   "The supported values vary per device",
			Exec:       c.mode,
		},
		{
			Name:       "power",
			ShortUsage: "power on|yes|off|no",
			Exec:       c.power,
		},
	}

	return &ffcli.Command{
		Name:        "control",
		ShortUsage:  "control [flags]",
		FlagSet:     fs,
		Subcommands: subcommands,
		ShortHelp:   "Control lets you send commands to a device",
		LongHelp: "The control command lets you send commands to a device. " +
			"This lets you change certain settings, like power, the brightness of " +
			"the ring, the device mode etc.",
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}
}

func (c *config) brightness(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v philips.Brightness
	switch dest {
	case "on":
		v = philips.Brightness100
	case "off":
		v = philips.Brightness0
	case "25":
		v = philips.Brightness25
	case "50":
		v = philips.Brightness50
	case "75":
		v = philips.Brightness75
	default:
		return flag.ErrHelp
	}

	err = cl.Set(&philips.Desired{Brightness: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for brigthness to: %s", dest)
	return nil
}

func (c *config) display(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v philips.DisplayMode
	switch dest {
	case "iaq":
		v = philips.IAQ
	case "humidity":
		v = philips.Humidity
	case "pm25":
		v = philips.PM25
	default:
		return flag.ErrHelp
	}

	err = cl.Set(&philips.Desired{DisplayMode: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for display mode to: %s", dest)
	return nil
}

func (c *config) fanspeed(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v philips.FanSpeed
	switch dest {
	case "silent":
		v = philips.Silent
	case "turbo":
		v = philips.Turbo
	case "1":
		v = philips.Speed1
	case "2":
		v = philips.Speed2
	case "3":
		v = philips.Speed3
	default:
		return flag.ErrHelp
	}

	err = cl.Set(&philips.Desired{FanSpeed: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for fan speed to: %s", dest)
	return nil
}

func (c *config) function(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v philips.Function
	switch dest {
	case "purification":
		v = philips.Purification
	case "humidification":
		v = philips.PurificationHumidification
	default:
		return flag.ErrHelp
	}

	err = cl.Set(&philips.Desired{Function: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for function speed to: %s", dest)
	return nil
}

func (c *config) humidity(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v int
	switch dest {
	case "40", "50", "60":
		v, err = strconv.Atoi(dest)
		if err != nil {
			return err
		}
	case "max":
		v = 70
	default:
		return flag.ErrHelp
	}

	err = cl.Set(&philips.Desired{RelativeHumidityTarget: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for humidity to: %s", dest)
	return nil
}

func (c *config) lock(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v bool
	switch dest {
	case "on", "yes":
		v = true
	default:
		v = false
	}

	err = cl.Set(&philips.Desired{ChildLock: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for (child)lock to: %s", dest)
	return nil
}

func (c *config) mode(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v philips.Mode
	switch dest {
	case "auto":
		v = philips.Auto
	case "allergen":
		v = philips.Allergen
	case "bacteria":
		v = philips.Bacteria
	case "manual":
		v = philips.Manual
	case "night":
		v = philips.Night
	case "sleep":
		v = philips.Sleep
	default:
		return flag.ErrHelp
	}

	err = cl.Set(&philips.Desired{Mode: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for mode to: %s", dest)
	return nil
}

func (c *config) power(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return flag.ErrHelp
	}

	cl, err := philips.New(ctx, c.host)
	if err != nil {
		return err
	}

	dest := strings.ToLower(args[0])
	var v philips.Power
	switch dest {
	case "on", "yes":
		v = philips.On
	default:
		v = philips.Off
	}

	err = cl.Set(&philips.Desired{Power: &v})
	if err != nil {
		return err
	}

	log.Printf("changed value for power to: %s", dest)
	return nil
}
