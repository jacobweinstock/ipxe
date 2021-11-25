package file

import (
	"context"
	"fmt"
	"net"

	"github.com/tinkerbell/tink/protos/hardware"
)

type File struct {
	DB []*hardware.Hardware
}

func (f File) Mac(_ context.Context, ip net.IP, mac net.HardwareAddr) (net.HardwareAddr, error) {
	for _, v := range f.DB {
		for _, hip := range v.Network.Interfaces {
			if net.ParseIP(hip.Dhcp.Ip.Address).Equal(ip) {
				return net.ParseMAC(hip.Dhcp.Mac)
			}
			if hw, err := net.ParseMAC(hip.Dhcp.Mac); err == nil {
				if hw.String() == mac.String() {
					return hw, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (f File) Allowed(_ context.Context, ip net.IP, mac net.HardwareAddr) (bool, error) {
	for _, v := range f.DB {
		for _, hip := range v.Network.Interfaces {
			if net.ParseIP(hip.Dhcp.Ip.Address).Equal(ip) {
				// TODO(jacobweinstock): handle hip.Netboot being nil, do we validate the config initially?
				return hip.Netboot.AllowPxe, nil
			}
			if hw, err := net.ParseMAC(hip.Dhcp.Mac); err == nil {
				if hw.String() == mac.String() {
					return hip.Netboot.AllowPxe, nil
				}
			}
		}
	}
	return false, nil
}
