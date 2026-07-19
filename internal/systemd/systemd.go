package systemd

import (
	"context"

	"github.com/godbus/dbus/v5"
)

const (
	path         = "/org/freedesktop/systemd1"
	service      = "org.freedesktop.systemd1"
	managerIFace = service + ".Manager"
	startUnit    = managerIFace + ".StartUnit"
	stopUnit     = managerIFace + ".StopUnit"
)

func StartUnit(ctx context.Context, conn *dbus.Conn, unit string) error {
	call := object(conn).CallWithContext(ctx, startUnit, 0, unit, "replace")
	return call.Err
}

func StopUnit(ctx context.Context, conn *dbus.Conn, unit string) error {
	call := object(conn).CallWithContext(ctx, stopUnit, 0, unit, "replace")
	return call.Err
}

func object(conn *dbus.Conn) dbus.BusObject {
	return conn.Object(service, path)
}
