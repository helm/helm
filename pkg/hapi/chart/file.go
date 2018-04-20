package chart

// File represents a file as a name/value pair.
//
// By convention, name is a relative path within the scope of the chart's
// base directory.
type File struct {
	// Name is the path-like name of the template.
	Name string `json:"name,omitempty"`
	// Data is the template as byte data.
	Data []byte `json:"data,omitempty"`
}
