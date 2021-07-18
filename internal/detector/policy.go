package detector

import (
	"context"
	"net"
)

type Policy interface {
	IsManaged() bool
	String() string
	GetIP4(context.Context) (net.IP, bool)
	GetIP6(context.Context) (net.IP, bool)
}
