package rules

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/lint/support"
	"testing"
	"text/template"
)

func TestFilterIgnoredMessages(t *testing.T) {
	type args struct {
		messages       []support.Message
		ignorePatterns map[string][]string
	}
	tests := []struct {
		name string
		args args
		want []support.Message
	}{
		{
			name: "should filter ignored messages only",
			args: args{
				messages: []support.Message{
					{
						Severity: 3,
						Path:     "templates/",
						Err:      template.ExecError{
							Name: "certmanager-issuer/templates/rbac-config.yaml",
							Err:  fmt.Errorf(`template: certmanager-issuer/templates/rbac-config.yaml:1:67: executing "certmanager-issuer/templates/rbac-config.yaml" at <.Values.global.ingress>: nil pointer evaluating interface {}.ingress`),
						},
					},
					{
						Severity: 1,
						Path:     "values.yaml",
						Err:      fmt.Errorf("file does not exist"),
					},
					{
						Severity: 1,
						Path:     "Chart.yaml",
						Err:      fmt.Errorf("icon is recommended"),
					},
				},
				ignorePatterns: map[string][]string{
					"certmanager-issuer/templates/rbac-config.yaml": {
						"<.Values.global.ingress>",
					},
				},
			},
			want: []support.Message{
				{
					Severity: 1,
					Path:     "values.yaml",
					Err:      fmt.Errorf("file does not exist"),
				},
				{
					Severity: 1,
					Path:     "Chart.yaml",
					Err:      fmt.Errorf("icon is recommended"),
				},

			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterIgnoredMessages(tt.args.messages, tt.args.ignorePatterns)
			assert.Equalf(t, tt.want, got, "FilterIgnoredMessages(%v, %v)", tt.args.messages, tt.args.ignorePatterns)
		})
	}
}
