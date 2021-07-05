package api

type Handler interface {
	Handle() (*Handle, error)
}

type TokenHandler struct {
	Token string
}

func (t *TokenHandler) Handle() (*Handle, error) {
	return newWithToken(t.Token)
}

/*
type KeyHandler struct {
	Key   string
	Email string
}

func (t *KeyHandler) Handle() (*Handle, error) {
	return newWithKey(t.Key, t.Email)
}
*/
