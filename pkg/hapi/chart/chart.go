package chart

// 	Chart is a helm package that contains metadata, a default config, zero or more
// 	optionally parameterizable templates, and zero or more charts (dependencies).
type Chart struct {
	// Contents of the Chartfile.
	Metadata *Metadata `json:"metadata,omitempty"`
	// Templates for this chart.
	Templates []*File `json:"templates,omitempty"`
	// Charts that this chart depends on.
	Dependencies []*Chart `json:"dependencies,omitempty"`
	// Default config for this template.
	Values []byte `json:"values,omitempty"`
	// Miscellaneous files in a chart archive,
	// e.g. README, LICENSE, etc.
	Files []*File `json:"files,omitempty"`
}
