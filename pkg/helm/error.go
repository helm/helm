package helm

const (
	errNotImplemented = Error("helm api not implemented")
	errMissingSrvAddr = Error("missing tiller address")
	errMissingTpls    = Error("missing chart templates")
	errMissingChart   = Error("missing chart metadata")
	errMissingValues  = Error("missing chart values")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
