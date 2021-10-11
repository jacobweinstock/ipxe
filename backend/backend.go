package backend

import (
	"context"
	"fmt"
	"net"
)

type Reader interface {
	Mac(context.Context, net.IP) (net.HardwareAddr, error) // seems to only be used for logging. might not need.
	Allowed(context.Context, net.IP) bool
}

type Tinkerbell struct{}

func (t Tinkerbell) Mac(ctx context.Context, ip net.IP) (net.HardwareAddr, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t Tinkerbell) Allowed(ctx context.Context, ip net.IP) bool {
	return false
}
