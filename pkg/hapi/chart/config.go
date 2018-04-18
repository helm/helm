package chart

// Config supplies values to the parametrizable templates of a chart.
type Config struct {
	Raw    string            `json:"raw,omitempty"`
	Values map[string]*Value `json:"values,omitempty"`
}

// Value describes a configuration value as a string.
type Value struct {
	Value string `json:"value,omitempty"`
}
