package api

type NewHandler interface {
	NewHandle() (*Handle, error)
}

type TokenNewHandler struct {
	Token string
}

func (t *TokenNewHandler) NewHandle() (*Handle, error) {
	return newWithToken(t.Token)
}
