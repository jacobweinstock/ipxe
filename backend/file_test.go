package backend

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tinkerbell/tink/protos/hardware"
)

func TestMac(t *testing.T) {
	mactests := map[string]struct {
		ip  string
		mac string
		err error
	}{
		"mac found":     {"192.168.2.3", "0a:00:27:00:00:00", nil},
		"mac not found": {"192.168.2.3", "00:00:00:00:00:00", fmt.Errorf("not found")},
	}
	for name, tc := range mactests {
		t.Run(name, func(t *testing.T) {
			record := File{DB: []*hardware.Hardware{{
				Network: &hardware.Hardware_Network{
					Interfaces: []*hardware.Hardware_Network_Interface{
						{Dhcp: &hardware.Hardware_DHCP{
							Ip: &hardware.Hardware_DHCP_IP{},
						}},
					},
				},
			}}}
			if tc.err == nil {
				record.DB[0].Network.Interfaces[0].Dhcp.Mac = tc.mac
				record.DB[0].Network.Interfaces[0].Dhcp.Ip.Address = tc.ip
			}
			want, _ := net.ParseMAC(tc.mac)
			hw, _ := net.ParseMAC(tc.mac)
			got, err := record.Mac(context.TODO(), net.ParseIP(tc.ip), hw)
			if err != nil {
				if tc.err == nil {
					t.Fatalf("expected err: nil, got: %v", err)
				}
				if cmp.Diff(tc.err.Error(), err.Error()) != "" {
					t.Fatalf("expected err: %v, got: %v", tc.err, err)
				}
			} else {
				if tc.err != nil {
					t.Fatalf("expected err: %v, got: nil", tc.err)
				}
				if cmp.Diff(got, want) != "" {
					t.Fatalf("got %s, want %s", got, want)
				}
			}
		})
	}
}

func TestAllowed(t *testing.T) {
	mactests := map[string]struct {
		ip      string
		mac     string
		allowed bool
		err     error
	}{
		"ip allowed":                   {"192.168.2.3", "0a:00:27:00:00:00", true, nil},
		"ip not allowed":               {"192.168.2.3", "0a:00:27:00:00:00", false, nil},
		"ip not found and not allowed": {"192.168.2.2", "0a:00:27:00:00:00", false, fmt.Errorf("not found")},
	}
	for name, tc := range mactests {
		t.Run(name, func(t *testing.T) {
			record := File{DB: []*hardware.Hardware{{
				Network: &hardware.Hardware_Network{
					Interfaces: []*hardware.Hardware_Network_Interface{
						{
							Dhcp: &hardware.Hardware_DHCP{
								Ip: &hardware.Hardware_DHCP_IP{},
							},
							Netboot: &hardware.Hardware_Netboot{},
						},
					},
				},
			}}}
			if tc.err == nil {
				record.DB[0].Network.Interfaces[0].Dhcp.Ip.Address = tc.ip
			}
			record.DB[0].Network.Interfaces[0].Netboot.AllowPxe = tc.allowed
			hw, _ := net.ParseMAC(tc.mac)
			got, err := record.Allowed(context.TODO(), net.ParseIP(tc.ip), hw)
			if err != nil {
				if tc.err == nil {
					t.Fatalf("expected err: nil, got: %v", err)
				}
				if cmp.Diff(tc.err.Error(), err.Error()) != "" {
					t.Fatalf("expected err: %v, got: %v", tc.err, err)
				}
			}
			if cmp.Diff(got, tc.allowed) != "" {
				t.Fatalf("got %v, want %v", got, tc.allowed)
			}
		})
	}
}
