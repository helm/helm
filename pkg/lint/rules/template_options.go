package rules

type (
	templateOptions struct {
		releaseName string
	}

	TemplateOption func(o *templateOptions)
)

var defaultTemplateOptions = &templateOptions{
	releaseName: "test-release",
}

// WithReleaseName specify release name for linter
func WithReleaseName(name string) TemplateOption {
	return func(o *templateOptions) {
		o.releaseName = name
	}
}
