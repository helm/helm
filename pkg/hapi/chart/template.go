package chart

// Template represents a template as a name/value pair.
//
// By convention, name is a relative path within the scope of the chart's
// base directory.
type Template struct {
	// Name is the path-like name of the template.
	Name string `json:"name,omitempty"`
	// Data is the template as byte data.
	Data []byte `json:"data,omitempty"`
}

func (m *Template) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Template) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}
