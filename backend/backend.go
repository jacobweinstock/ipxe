package backend

import (
	"context"
	"net"
)

type Reader interface {
	Mac(context.Context, net.IP, net.HardwareAddr) (net.HardwareAddr, error) // seems to only be used for logging. might not need.
	Allowed(context.Context, net.IP, net.HardwareAddr) (bool, error)
}
