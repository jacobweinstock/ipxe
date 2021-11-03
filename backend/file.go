package backend

import (
	"context"
	"fmt"
	"net"

	"github.com/tinkerbell/tink/protos/hardware"
)

type File struct {
	DB []*hardware.Hardware
}

func (f File) Mac(_ context.Context, ip net.IP) (net.HardwareAddr, error) {
	for _, v := range f.DB {
		for _, hip := range v.Network.Interfaces {
			if net.ParseIP(hip.Dhcp.Ip.Address).Equal(ip) {
				return net.ParseMAC(hip.Dhcp.Mac)
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (f File) Allowed(_ context.Context, ip net.IP) (bool, error) {
	for _, v := range f.DB {
		for _, hip := range v.Network.Interfaces {
			if net.ParseIP(hip.Dhcp.Ip.Address).Equal(ip) {
				// TODO(jacobweinstock): handle hip.Netboot being nil, do we validate the config initially?
				return hip.Netboot.AllowPxe, nil
			}
		}
	}
	return false, nil
}
