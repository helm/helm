package helm

const (
	// ErrNotImplemented indicates that this API is not implemented.
	ErrNotImplemented = Error("helm api not implemented")
	// ErrInvalidSrvAddr indicates an invalid address to the Tiller server.
	ErrInvalidSrvAddr = Error("invalid tiller address")
	// ErrMissingTpls indicates that the templates are missing from a chart.
	ErrMissingTpls = Error("missing chart templates")
	// ErrMissingChart indicates that the Chart.yaml data is missing.
	ErrMissingChart = Error("missing chart metadata")
	// ErrMissingValues indicates that the config values.yaml data is missing.
	ErrMissingValues = Error("missing chart values")
)

// Error represents a Helm client error.
type Error string

// Error returns a string representation of this error.
func (e Error) Error() string {
	return string(e)
}
