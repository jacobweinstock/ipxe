package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
)

type File struct {
	Filename string
	db       []*hardware.Hardware
}

func (f File) Mac(_ context.Context, ip net.IP) (net.HardwareAddr, error) {
	for _, v := range f.db {
		for _, hip := range v.Network.Interfaces {
			if net.ParseIP(hip.Dhcp.Ip.Address).Equal(ip) {
				return net.HardwareAddr(hip.Dhcp.Mac), nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (f File) Allowed(_ context.Context, ip net.IP) (bool, error) {
	for _, v := range f.db {
		for _, hip := range v.Network.Interfaces {
			if net.ParseIP(hip.Dhcp.Ip.Address).Equal(ip) {
				return hip.Netboot.AllowPxe, nil
			}
		}
	}
	return false, nil
}

func NewFile(f string) (*File, error) {
	saData, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read file %q", f)
	}
	dsDB := []*hardware.Hardware{}
	if err := json.Unmarshal(saData, &dsDB); err != nil {
		return nil, errors.Wrapf(err, "unable to parse configuration file %q", f)
	}

	return &File{
		Filename: f,
		db:       dsDB,
	}, nil
}
