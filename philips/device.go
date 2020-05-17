package philips

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-ocf/go-coap"
)

// Device represents a AirCombi device that you can talk to
type Device struct {
	addr string
	cc   *coap.ClientConn
	ctx  context.Context
	id   *Session
}

// New returns a CoAP client configured to talk to a device
func New(ctx context.Context, address string) (*Device, error) {
	cl := coap.Client{
		Net:         "udp",
		DialTimeout: 5 * time.Second,
		// Internally the time is divided by 6, so this results in a ping/pong every 5s
		// which is what the Air Matters app does
		KeepAlive: coap.MustMakeKeepAlive(30 * time.Second),
	}

	conn, err := cl.DialWithContext(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("error dialing: %w", err)
	}

	d := &Device{
		cc:   conn,
		ctx:  ctx,
		addr: address,
	}

	sess := NewSession()
	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()

	rsp, err := d.cc.PostWithContext(ctx, "/sys/dev/sync", coap.TextPlain, bytes.NewReader([]byte(sess.Hex())))
	if err != nil {
		return nil, fmt.Errorf("failed to post to /sys/dev/sync and get session: %w", err)
	}

	id := ParseID(rsp.Payload())
	id.Increment()
	d.id = id

	return d, nil
}

// Info returns the decoded payload from /sys/dev/info
func (d *Device) Info() (*Info, error) {
	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()

	devInfo, err := d.cc.GetWithContext(ctx, "/sys/dev/info")
	if err != nil {
		return nil, fmt.Errorf("failed to get /sys/dev/info: %w", err)
	}

	var info Info
	if err := json.Unmarshal(devInfo.Payload(), &info); err != nil {
		return nil, fmt.Errorf("could not decode info: %w", err)
	}
	return &info, nil
}

// Set lets you set a certain attribute of the device to its desired state.
// This lets you do things like turn the device on and off.
//
// It's worth noting that the device also returns a "status: success" if you
// send it complete nonesense that it couldn't make sense of, instead of a
// failure. As such the error returned here is somewhat dubious and the caller
// needs to know what they're doing for this to be at all useful. Aka, you have
// to test in production.
//
// Also, doing something like turning the device on while it is already on
// equally returns success.
func (d *Device) Set(msg *Desired) error {
	data, err := json.Marshal(
		Status{
			State: State{
				Desired: msg,
			},
		},
	)
	if err != nil {
		return err
	}

	newMsg, err := EncodeMessage(d.id, []byte(data))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()

	resp, err := d.cc.PostWithContext(ctx, "/sys/dev/control", coap.AppJSON, bytes.NewReader(newMsg))
	if err != nil {
		return err
	}
	d.id.Increment()

	state := map[string]string{}
	err = json.Unmarshal(resp.Payload(), &state)
	if err != nil {
		return err
	}

	if state["status"] != "success" {
		return fmt.Errorf("did not manage to set value")
	}
	return nil
}

// Status lets you subcrivbe to /sys/dev/status and get updates as the
// devices has them. You should call Cancel() on the observation once
// you're done with it
func (d *Device) Status(callback func(req *coap.Request)) (*coap.Observation, error) {
	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()

	obs, err := d.cc.ObserveWithContext(ctx, "/sys/dev/status", callback)
	if err != nil {
		return nil, fmt.Errorf("failed to start observe on /sys/dev/status: %w", err)
	}
	return obs, nil
}

// CoAPClient lets you access the underlying CoAP connection in case you need
// to do something manually
func (d *Device) CoAPClient() *coap.ClientConn {
	return d.cc
}
