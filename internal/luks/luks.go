// Package luks manages LUKS-encrypted volumes on removable USB devices
// through the udisks2 D-Bus API. Privileged operations are authorized by the
// appliance's polkit rules (non-"-system" udisks2 action ids), so no root or
// sudo is required.
//
// Mutating operations are guarded by checkDevice, which requires a removable
// USB drive that is not mounted outside allowedMountRoots. Init accepts a
// force argument that bypasses the guard; nothing else does.
package luks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	service = "org.freedesktop.UDisks2"

	rootPath    = dbus.ObjectPath("/org/freedesktop/UDisks2")
	managerPath = dbus.ObjectPath("/org/freedesktop/UDisks2/Manager")

	ifaceManager        = "org.freedesktop.UDisks2.Manager"
	ifaceBlock          = "org.freedesktop.UDisks2.Block"
	ifaceDrive          = "org.freedesktop.UDisks2.Drive"
	ifaceEncrypted      = "org.freedesktop.UDisks2.Encrypted"
	ifaceFilesystem     = "org.freedesktop.UDisks2.Filesystem"
	ifacePartition      = "org.freedesktop.UDisks2.Partition"
	ifacePartitionTable = "org.freedesktop.UDisks2.PartitionTable"

	ifaceObjectManager = "org.freedesktop.DBus.ObjectManager"
	ifaceProperties    = "org.freedesktop.DBus.Properties"
)

// managedObjects mirrors the a{oa{sa{sv}}} result of GetManagedObjects:
// object path -> interface -> property -> value.
type managedObjects = map[dbus.ObjectPath]map[string]map[string]dbus.Variant

// Client talks to udisks2 on the system bus.
type Client struct {
	conn *dbus.Conn
}

// NewClient connects to the system bus.
func NewClient() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("could not connect to system bus: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

// Device describes one removable USB block device.
type Device struct {
	// Device is the special device file, e.g. /dev/sda.
	Device string
	// Size is the device size in bytes.
	Size uint64
	// Model is the drive vendor/model string.
	Model string
	// Type is the detected content type (e.g. ext4, crypto_LUKS).
	Type string
	// Label is the filesystem/container label, if any.
	Label string
	// Partition reports whether this is a partition rather than a whole
	// device.
	Partition bool
	// Encrypted reports whether the device is a LUKS container.
	Encrypted bool
	// Unlocked reports whether an encrypted device currently has a cleartext
	// mapping.
	Unlocked bool
	// MountPoints are the active mountpoints (of the cleartext device when
	// the container is unlocked).
	MountPoints []string
}

// List returns all attached removable USB block devices. Cleartext mappings
// of unlocked containers are folded into their backing device.
func (c *Client) List(ctx context.Context) ([]Device, error) {
	objs, err := c.managedObjects(ctx)
	if err != nil {
		return nil, err
	}

	var devices []Device
	for path, ifaces := range objs {
		block, ok := ifaces[ifaceBlock]
		if !ok || !usbDevice(objs, path) {
			continue
		}
		if p := variant[dbus.ObjectPath](block, "CryptoBackingDevice"); !nullPath(p) {
			continue
		}

		_, isPartition := ifaces[ifacePartition]
		d := Device{
			Device:    cString(variant[[]byte](block, "Device")),
			Size:      variant[uint64](block, "Size"),
			Type:      variant[string](block, "IdType"),
			Label:     variant[string](block, "IdLabel"),
			Partition: isPartition,
		}
		if pt, ok := ifaces[ifacePartitionTable]; ok && d.Type == "" {
			d.Type = variant[string](pt, "Type") + " partition table"
		}
		if drive, ok := objs[variant[dbus.ObjectPath](block, "Drive")][ifaceDrive]; ok {
			d.Model = strings.TrimSpace(variant[string](drive, "Vendor") + " " + variant[string](drive, "Model"))
		}
		if enc, ok := ifaces[ifaceEncrypted]; ok {
			d.Encrypted = true
			if clear := variant[dbus.ObjectPath](enc, "CleartextDevice"); clear != "" && clear != "/" {
				d.Unlocked = true
				d.MountPoints = mountPoints(objs[clear][ifaceFilesystem])
			}
		} else if fs, ok := ifaces[ifaceFilesystem]; ok {
			d.MountPoints = mountPoints(fs)
		}
		devices = append(devices, d)
	}

	slices.SortFunc(devices, func(a, b Device) int { return strings.Compare(a.Device, b.Device) })
	return devices, nil
}

// Init formats the device as a LUKS2 container holding an ext4 filesystem
// owned by the calling user, then mounts it, returning the mountpoint. This
// DESTROYS all data on the device. The passphrase travels over D-Bus, never
// through argv. A non-empty label names the filesystem, which also makes the
// mountpoint deterministic (/run/media/<user>/<label>).
func (c *Client) Init(ctx context.Context, device, passphrase, label string, force bool) (string, error) {
	path, _, err := c.resolve(ctx, device, force)
	if err != nil {
		return "", err
	}
	options := map[string]dbus.Variant{
		"encrypt.passphrase": dbus.MakeVariant(passphrase),
		"encrypt.type":       dbus.MakeVariant("luks2"),
		"take-ownership":     dbus.MakeVariant(true),
	}
	if label != "" {
		options["label"] = dbus.MakeVariant(label)
	}
	call := c.conn.Object(service, path).CallWithContext(ctx, ifaceBlock+".Format", 0, "ext4", options)
	if call.Err != nil {
		return "", fmt.Errorf("could not format %s: %w", device, call.Err)
	}
	// Format leaves the container unlocked; lock and re-unlock with the
	// supplied passphrase anyway to prove it opens the container before
	// anything is stored on it.
	objs, err := c.managedObjects(ctx)
	if err != nil {
		return "", err
	}
	if err := c.lock(ctx, path, objs, device); err != nil {
		return "", err
	}
	if objs, err = c.managedObjects(ctx); err != nil {
		return "", err
	}
	return c.unlock(ctx, path, objs, device, func() (string, error) { return passphrase, nil })
}

// Unlock opens the LUKS container and mounts its filesystem, returning the
// mountpoint. The passphrase is only requested when the container is
// actually locked; unlocking or mounting is skipped when already done.
func (c *Client) Unlock(ctx context.Context, device string, passphrase func() (string, error)) (string, error) {
	path, objs, err := c.resolve(ctx, device, false)
	if err != nil {
		return "", err
	}
	return c.unlock(ctx, path, objs, device, passphrase)
}

func (c *Client) unlock(ctx context.Context, path dbus.ObjectPath, objs managedObjects, device string, passphrase func() (string, error)) (string, error) {
	enc, ok := objs[path][ifaceEncrypted]
	if !ok {
		return "", fmt.Errorf("%s is not a LUKS device", device)
	}

	clear := variant[dbus.ObjectPath](enc, "CleartextDevice")
	if nullPath(clear) {
		pass, err := passphrase()
		if err != nil {
			return "", err
		}
		call := c.conn.Object(service, path).CallWithContext(ctx, ifaceEncrypted+".Unlock", 0, pass, map[string]dbus.Variant{})
		if call.Err != nil {
			return "", fmt.Errorf("could not unlock %s: %w", device, call.Err)
		}
		if err := call.Store(&clear); err != nil {
			return "", err
		}
		return c.mount(ctx, clear, nil)
	}

	return c.mount(ctx, clear, objs[clear][ifaceFilesystem])
}

// Lock unmounts the cleartext filesystem (if mounted) and locks the LUKS
// container. Locking an already-locked device is a no-op.
func (c *Client) Lock(ctx context.Context, device string) error {
	path, objs, err := c.resolve(ctx, device, false)
	if err != nil {
		return err
	}
	return c.lock(ctx, path, objs, device)
}

func (c *Client) lock(ctx context.Context, path dbus.ObjectPath, objs managedObjects, device string) error {
	enc, ok := objs[path][ifaceEncrypted]
	if !ok {
		return fmt.Errorf("%s is not a LUKS device", device)
	}

	clear := variant[dbus.ObjectPath](enc, "CleartextDevice")
	if nullPath(clear) {
		return nil // already locked
	}
	if len(mountPoints(objs[clear][ifaceFilesystem])) > 0 {
		call := c.conn.Object(service, clear).CallWithContext(ctx, ifaceFilesystem+".Unmount", 0, map[string]dbus.Variant{})
		if call.Err != nil {
			return fmt.Errorf("could not unmount %s: %w", device, call.Err)
		}
	}
	call := c.conn.Object(service, path).CallWithContext(ctx, ifaceEncrypted+".Lock", 0, map[string]dbus.Variant{})
	if call.Err != nil {
		return fmt.Errorf("could not lock %s: %w", device, call.Err)
	}
	return nil
}

// mount mounts the filesystem on the given block object and returns the
// mountpoint, reusing an existing mountpoint if there is one. fs holds the
// object's Filesystem properties from an existing snapshot; pass nil to read
// them off the bus.
func (c *Client) mount(ctx context.Context, path dbus.ObjectPath, fs map[string]dbus.Variant) (string, error) {
	obj := c.conn.Object(service, path)
	if fs == nil {
		fs = c.filesystemProps(ctx, obj)
	}
	if pts := mountPoints(fs); len(pts) > 0 {
		return pts[0], nil
	}

	// the Filesystem interface can lag behind Unlock while udisks2 probes the
	// cleartext device, so retry briefly before giving up
	var call *dbus.Call
	for range 10 {
		call = obj.CallWithContext(ctx, ifaceFilesystem+".Mount", 0, map[string]dbus.Variant{})
		if call.Err == nil {
			var mountpoint string
			if err := call.Store(&mountpoint); err != nil {
				return "", err
			}
			return mountpoint, nil
		}
		if !notProbedYet(call.Err) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("could not mount %s: %w", path, call.Err)
}

// filesystemProps reads a block object's Filesystem properties, returning nil
// when the interface is missing or has not been probed yet.
func (c *Client) filesystemProps(ctx context.Context, obj dbus.BusObject) map[string]dbus.Variant {
	call := obj.CallWithContext(ctx, ifaceProperties+".GetAll", 0, ifaceFilesystem)
	if call.Err != nil {
		return nil
	}
	var props map[string]dbus.Variant
	if err := call.Store(&props); err != nil {
		return nil
	}
	return props
}

// resolve resolves a device special file (e.g. /dev/sda) to its udisks2 object
// path and refuses anything that is not a safe removable USB device. force
// bypasses that check, which can destroy the running OS disk.
func (c *Client) resolve(ctx context.Context, device string, force bool) (dbus.ObjectPath, managedObjects, error) {
	call := c.conn.Object(service, managerPath).CallWithContext(ctx, ifaceManager+".ResolveDevice", 0,
		map[string]dbus.Variant{"path": dbus.MakeVariant(device)},
		map[string]dbus.Variant{},
	)
	if call.Err != nil {
		return "", nil, fmt.Errorf("could not resolve %s: %w", device, call.Err)
	}
	var paths []dbus.ObjectPath
	if err := call.Store(&paths); err != nil {
		return "", nil, err
	}
	if len(paths) == 0 {
		return "", nil, fmt.Errorf("no such device: %s", device)
	}

	objs, err := c.managedObjects(ctx)
	if err != nil {
		return "", nil, err
	}
	if err := checkDevice(objs, paths[0]); err != nil {
		if !force {
			return "", nil, fmt.Errorf("refusing to touch %s: %w (pass --force to override)", device, err)
		}
		slog.Warn("safety check overridden by --force",
			slog.String("device", device), slog.Any("reason", err))
	}
	return paths[0], objs, nil
}

func (c *Client) managedObjects(ctx context.Context) (managedObjects, error) {
	call := c.conn.Object(service, rootPath).CallWithContext(ctx, ifaceObjectManager+".GetManagedObjects", 0)
	if call.Err != nil {
		return nil, fmt.Errorf("could not enumerate udisks2 objects: %w", call.Err)
	}
	var objs managedObjects
	if err := call.Store(&objs); err != nil {
		return nil, err
	}
	return objs, nil
}

// allowedMountRoots are the only locations a device this package will touch
// may be mounted at. An allowlist rather than a denylist of system paths:
// HintSystem and ConnectionBus do not distinguish a USB stick from a
// USB-attached boot disk (udisks2 clears HintSystem for anything on the USB
// bus), so where a device is *currently mounted* is the load-bearing signal
// that it belongs to the running OS.
var allowedMountRoots = []string{"/run/media/", "/mnt/", "/media/"}

// usbDevice reports whether the block object sits on a removable USB drive.
// This is the read-only filter used by List; mutating operations additionally
// go through checkDevice.
func usbDevice(objs managedObjects, path dbus.ObjectPath) bool {
	block, ok := objs[path][ifaceBlock]
	if !ok {
		return false
	}
	if variant[bool](block, "HintSystem") {
		return false
	}
	drive := variant[dbus.ObjectPath](block, "Drive")
	if nullPath(drive) {
		return false
	}
	props, ok := objs[drive][ifaceDrive]
	if !ok {
		return false
	}
	return variant[string](props, "ConnectionBus") == "usb" && variant[bool](props, "Removable")
}

// checkDevice returns nil only when the block object is safe for this package
// to modify: a removable USB drive that is not mounted anywhere the running OS
// depends on. The returned error names the specific reason.
func checkDevice(objs managedObjects, path dbus.ObjectPath) error {
	block, ok := objs[path][ifaceBlock]
	if !ok {
		return errors.New("no such udisks2 block device")
	}
	if variant[bool](block, "HintSystem") {
		return errors.New("device is hinted as a system device")
	}
	drive := variant[dbus.ObjectPath](block, "Drive")
	if nullPath(drive) {
		return errors.New("device has no backing drive")
	}
	props, ok := objs[drive][ifaceDrive]
	if !ok {
		return errors.New("device has no backing drive")
	}
	if bus := variant[string](props, "ConnectionBus"); bus != "usb" {
		if bus == "" {
			bus = "an internal bus"
		}
		return fmt.Errorf("device is attached over %s, not usb", bus)
	}
	if !variant[bool](props, "Removable") {
		return errors.New("drive is not removable")
	}
	if mp := disallowedMount(objs, path); mp != "" {
		return fmt.Errorf("device is mounted at %s, which the running system may depend on", mp)
	}
	return nil
}

// disallowedMount returns the first mountpoint of the device, its partitions,
// or any cleartext mapping of either that falls outside allowedMountRoots.
func disallowedMount(objs managedObjects, path dbus.ObjectPath) string {
	for _, p := range append(descendants(objs, path), path) {
		for _, mp := range mountPoints(objs[p][ifaceFilesystem]) {
			if !slices.ContainsFunc(allowedMountRoots, func(root string) bool {
				return strings.HasPrefix(mp, root)
			}) {
				return mp
			}
		}
	}
	return ""
}

// descendants returns every object layered on top of path: its partitions, any
// cleartext mapping backed by it, and the same for each of those in turn.
func descendants(objs managedObjects, path dbus.ObjectPath) []dbus.ObjectPath {
	var out []dbus.ObjectPath
	for p, ifaces := range objs {
		block, ok := ifaces[ifaceBlock]
		if !ok || p == path {
			continue
		}
		if variant[dbus.ObjectPath](ifaces[ifacePartition], "Table") == path ||
			variant[dbus.ObjectPath](block, "CryptoBackingDevice") == path {
			out = append(out, p)
			out = append(out, descendants(objs, p)...)
		}
	}
	return out
}

// nullPath reports whether p is udisks2's "no object" sentinel.
func nullPath(p dbus.ObjectPath) bool { return p == "" || p == "/" }

// notProbedYet reports whether the error means the object's interface has not
// appeared on the bus yet.
func notProbedYet(err error) bool {
	var derr dbus.Error
	if !errors.As(err, &derr) {
		return false
	}
	switch derr.Name {
	case "org.freedesktop.DBus.Error.UnknownMethod",
		"org.freedesktop.DBus.Error.UnknownInterface",
		"org.freedesktop.DBus.Error.UnknownObject":
		return true
	}
	return false
}

// variant returns the property's value as T, or T's zero value when the
// property is missing or has a different type.
func variant[T any](props map[string]dbus.Variant, name string) T {
	var zero T
	v, ok := props[name]
	if !ok {
		return zero
	}
	return variantValue[T](v)
}

func variantValue[T any](v dbus.Variant) T {
	t, _ := v.Value().(T)
	return t
}

func mountPoints(fs map[string]dbus.Variant) []string {
	return byteStrings(variant[[][]byte](fs, "MountPoints"))
}

// byteStrings converts udisks2's NUL-terminated byte arrays to strings.
func byteStrings(bs [][]byte) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, cString(b))
	}
	return out
}

func cString(b []byte) string {
	return string(bytes.TrimRight(b, "\x00"))
}
