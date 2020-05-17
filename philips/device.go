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

	return &Device{
		cc:   conn,
		ctx:  ctx,
		addr: address,
	}, nil
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

// Session returns the session ID. It must be called once, before you
// do an Observe or want to send a command. After sending a command you
// have to increment the ID
func (d *Device) Session() (SessionID, error) {
	sess := NewID()
	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()

	rsp, err := d.cc.PostWithContext(ctx, "/sys/dev/sync", coap.TextPlain, bytes.NewReader([]byte(sess.Hex())))
	if err != nil {
		return 0, fmt.Errorf("failed to post to /sys/dev/sync and get session: %w", err)
	}

	id := ParseID(rsp.Payload()) + 1

	return id, nil
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
