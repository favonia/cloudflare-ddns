package detector

import (
	"net"
	"time"
)

type Policy interface {
	IsManaged() bool
	String() string
	GetIP4(time.Duration) (net.IP, error)
	GetIP6(time.Duration) (net.IP, error)
}
