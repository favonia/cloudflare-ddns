package common

type Policy int

const (
	Unmanaged Policy = iota
	Cloudflare
	Local
)

func (p Policy) String() string {
	switch p {
	case Unmanaged:
		return "unmanaged"
	case Cloudflare:
		return "cloudflare"
	case Local:
		return "local"
	default:
		return "<unrecognized>"
	}
}
