package detector

import (
	"context"
	"net"
)

type Policy interface {
	IsManaged() bool
	String() string
	GetIP(context.Context) (net.IP, bool)
}
