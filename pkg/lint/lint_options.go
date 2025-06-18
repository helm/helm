package lint

import (
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/lint/support"
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

func WithSkipSchemaValidation(enabled bool) LinterOption {
	return func(linter *support.Linter) {
		linter.SkipSchemaValidation = enabled
	}
}
