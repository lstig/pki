package luks

import (
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// props is shorthand for a udisks2 interface's property map.
type props map[string]any

// objects builds a managedObjects tree from a nested literal, wrapping every
// leaf value in a dbus.Variant the way GetManagedObjects would.
func objects(spec map[dbus.ObjectPath]map[string]props) managedObjects {
	objs := managedObjects{}
	for path, ifaces := range spec {
		objs[path] = map[string]map[string]dbus.Variant{}
		for iface, p := range ifaces {
			vs := map[string]dbus.Variant{}
			for k, v := range p {
				vs[k] = dbus.MakeVariant(v)
			}
			objs[path][iface] = vs
		}
	}
	return objs
}

// usbStick is the happy path: a removable USB drive with one unmounted
// partition on it.
func usbStick() map[dbus.ObjectPath]map[string]props {
	return map[dbus.ObjectPath]map[string]props{
		"/drive/usb": {ifaceDrive: {"ConnectionBus": "usb", "Removable": true}},
		"/block/sdb": {ifaceBlock: {"Drive": dbus.ObjectPath("/drive/usb"), "HintSystem": false}},
	}
}

func TestCheckDeviceAccepts(t *testing.T) {
	if err := checkDevice(objects(usbStick()), "/block/sdb"); err != nil {
		t.Fatalf("checkDevice rejected a plain USB stick: %v", err)
	}
}

func TestCheckDeviceRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[dbus.ObjectPath]map[string]props)
		wantErr string
	}{
		{
			name:    "hinted as a system device",
			mutate:  func(s map[dbus.ObjectPath]map[string]props) { s["/block/sdb"][ifaceBlock]["HintSystem"] = true },
			wantErr: "system device",
		},
		{
			name: "attached over a non-usb bus",
			mutate: func(s map[dbus.ObjectPath]map[string]props) {
				s["/drive/usb"][ifaceDrive]["ConnectionBus"] = "sata"
			},
			wantErr: "not usb",
		},
		{
			name: "non-removable USB-attached drive",
			mutate: func(s map[dbus.ObjectPath]map[string]props) {
				s["/drive/usb"][ifaceDrive]["Removable"] = false
			},
			wantErr: "not removable",
		},
		{
			name: "device itself mounted at /",
			mutate: func(s map[dbus.ObjectPath]map[string]props) {
				s["/block/sdb"][ifaceFilesystem] = props{"MountPoints": [][]byte{[]byte("/\x00")}}
			},
			wantErr: "mounted at /",
		},
		{
			name: "partition mounted at /boot",
			mutate: func(s map[dbus.ObjectPath]map[string]props) {
				s["/block/sdb1"] = map[string]props{
					ifaceBlock:      {"Drive": dbus.ObjectPath("/drive/usb")},
					ifacePartition:  {"Table": dbus.ObjectPath("/block/sdb")},
					ifaceFilesystem: {"MountPoints": [][]byte{[]byte("/boot\x00")}},
				}
			},
			wantErr: "mounted at /boot",
		},
		{
			name: "cleartext mapping mounted outside the allowed roots",
			mutate: func(s map[dbus.ObjectPath]map[string]props) {
				s["/block/dm0"] = map[string]props{
					ifaceBlock:      {"CryptoBackingDevice": dbus.ObjectPath("/block/sdb")},
					ifaceFilesystem: {"MountPoints": [][]byte{[]byte("/var/lib/thing\x00")}},
				}
			},
			wantErr: "mounted at /var/lib/thing",
		},
		{
			name:    "no backing drive",
			mutate:  func(s map[dbus.ObjectPath]map[string]props) { delete(s["/block/sdb"][ifaceBlock], "Drive") },
			wantErr: "no backing drive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := usbStick()
			tt.mutate(spec)
			err := checkDevice(objects(spec), "/block/sdb")
			if err == nil {
				t.Fatal("checkDevice accepted an unsafe device")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestCheckDeviceAllowsRemovableMounts confirms the guard is an allowlist of
// mount roots, not a denylist of system paths.
func TestCheckDeviceAllowsRemovableMounts(t *testing.T) {
	for _, mp := range []string{"/run/media/lstig/pki", "/mnt/usb", "/media/stick"} {
		spec := usbStick()
		spec["/block/sdb"][ifaceFilesystem] = props{"MountPoints": [][]byte{[]byte(mp + "\x00")}}
		if err := checkDevice(objects(spec), "/block/sdb"); err != nil {
			t.Errorf("checkDevice rejected a stick mounted at %s: %v", mp, err)
		}
	}
}

// TestUSBDeviceIgnoresMounts confirms List's filter stays loose: a device the
// mutating guard would refuse still shows up.
func TestUSBDeviceIgnoresMounts(t *testing.T) {
	spec := usbStick()
	spec["/block/sdb"][ifaceFilesystem] = props{"MountPoints": [][]byte{[]byte("/\x00")}}
	objs := objects(spec)

	if !usbDevice(objs, "/block/sdb") {
		t.Error("usbDevice hid a mounted USB device from list")
	}
	if err := checkDevice(objs, "/block/sdb"); err == nil {
		t.Error("checkDevice accepted a device mounted at /")
	}
}

func TestCString(t *testing.T) {
	if got := cString([]byte("/dev/sdb\x00")); got != "/dev/sdb" {
		t.Errorf("cString = %q, want %q", got, "/dev/sdb")
	}
}
