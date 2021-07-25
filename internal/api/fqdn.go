package api

type FQDN string

func (t FQDN) String() string {
	return string(t)
}
