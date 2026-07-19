package dbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	systemdService = "org.freedesktop.systemd1"
	systemdPath    = "/org/freedesktop/systemd1"
	ifaceSystemd   = "org.freedesktop.systemd1.Manager"
)

// Client wraps a system-bus connection to systemd.
type Client struct {
	conn *dbus.Conn
}

// Dial connects to the system bus.
func Dial() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect to system bus: %w", err)
	}
	return &Client{conn: conn}, nil
}

// Close releases the bus connection.
func (c *Client) Close() error { return c.conn.Close() }

// Start a systemd unit
func (c *Client) Start(unit string) error { return c.systemdJob("StartUnit", unit) }

// Stop a systemd unit
func (c *Client) Stop(unit string) error { return c.systemdJob("StopUnit", unit) }

func (c *Client) systemdJob(method, unit string) error {
	obj := c.conn.Object(systemdService, systemdPath)
	if err := obj.Call(ifaceSystemd+"."+method, 0, unit, "replace").Store(nil); err != nil {
		return fmt.Errorf("%s %s: %w", method, unit, err)
	}
	return nil
}
