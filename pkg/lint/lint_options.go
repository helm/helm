package lint

import (
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/lint/support"
)

type LinterOption func(linter *support.Linter)

// WithReleaseName specifies chart release name
func WithReleaseName(name string) LinterOption {
	return func(linter *support.Linter) {
		linter.ReleaseName = name
	}
}

// WithKubeVersion specifies kube version
func WithKubeVersion(version *chartutil.KubeVersion) LinterOption {
	return func(linter *support.Linter) {
		linter.KubeVersion = version
	}
}
