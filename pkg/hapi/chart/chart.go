package chart

import google_protobuf "github.com/golang/protobuf/ptypes/any"

// 	Chart is a helm package that contains metadata, a default config, zero or more
// 	optionally parameterizable templates, and zero or more charts (dependencies).
type Chart struct {
	// Contents of the Chartfile.
	Metadata *Metadata `json:"metadata,omitempty"`
	// Templates for this chart.
	Templates []*Template `json:"templates,omitempty"`
	// Charts that this chart depends on.
	Dependencies []*Chart `json:"dependencies,omitempty"`
	// Default config for this template.
	Values *Config `json:"values,omitempty"`
	// Miscellaneous files in a chart archive,
	// e.g. README, LICENSE, etc.
	Files []*google_protobuf.Any `json:"files,omitempty"`
}

func (m *Chart) GetMetadata() *Metadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *Chart) GetTemplates() []*Template {
	if m != nil {
		return m.Templates
	}
	return nil
}

func (m *Chart) GetDependencies() []*Chart {
	if m != nil {
		return m.Dependencies
	}
	return nil
}

func (m *Chart) GetValues() *Config {
	if m != nil {
		return m.Values
	}
	return nil
}

func (m *Chart) GetFiles() []*google_protobuf.Any {
	if m != nil {
		return m.Files
	}
	return nil
}
