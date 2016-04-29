package helm

const (
	ErrNotImplemented = Error("helm api not implemented")
	ErrInvalidSrvAddr = Error("invalid tiller address")
	ErrMissingTpls    = Error("missing chart templates")
	ErrMissingChart   = Error("missing chart metadata")
	ErrMissingValues  = Error("missing chart values")
)

// Error represents a Helm client error.
type Error string

func (e Error) Error() string {
	return string(e)
}
