package ignore

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/lint/support"
	"strings"
	"testing"
)

func TestRule_ShouldKeepMessage(t *testing.T) {
	type testCase struct {
		Scenario   string
		RuleText   string
		Ignorables []support.Message
	}

	testCases := []testCase{
		{
			Scenario: "subchart template not defined",
			RuleText: "gitlab/charts/webservice/templates/tests/tests.yaml <{{template \"fullname\" .}}>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: gitlab/charts/webservice/templates/tests/tests.yaml:5:20: executing \"gitlab/charts/webservice/templates/tests/tests.yaml\" at <{{template \"fullname\" .}}>: template \"fullname\" not defined"),
			}},
		}, {
			Scenario: "subchart template include template not found",
			RuleText: "gitaly/templates/statefulset.yml <include \"gitlab.gitaly.includeInternalResources\" $>\n",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: gitaly/templates/statefulset.yml:1:11: executing \"gitaly/templates/statefulset.yml\" at <include \"gitlab.gitaly.includeInternalResources\" $>: error calling include: template: no template \"gitlab.gitaly.includeInternalResources\" associated with template \"gotpl\""),
			}},
		},
		{
			Scenario: "subchart template evaluation has a nil pointer",
			RuleText: "gitlab-exporter/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>\n",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: gitlab-exporter/templates/serviceaccount.yaml:1:57: executing \"gitlab-exporter/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "webservice path only",
			RuleText: "webservice/templates/tests/tests.yaml",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: webservice/templates/tests/tests.yaml:8:8: executing \"webservice/templates/tests/tests.yaml\" at <include \"gitlab.standardLabels\" .>: error calling include: template: no template \"gitlab.standardLabels\" associated with template \"gotpl\""),
			}},
		}, {
			Scenario: "geo-logcursor path only",
			RuleText: "geo-logcursor/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: geo-logcursor/templates/serviceaccount.yaml:1:57: executing \"geo-logcursor/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "webservice path only",
			RuleText: "webservice/templates/service.yaml <include \"fullname\" .>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: gitlab/charts/gitlab/charts/webservice/templates/service.yaml:14:11: executing \"gitlab/charts/gitlab/charts/webservice/templates/service.yaml\" at <include \"fullname\" .>: error calling include: template: gitlab/templates/_helpers.tpl:14:27: executing \"fullname\" at <.Chart.Name>: nil pointer evaluating interface {}.Name"),
			}},
		}, {
			Scenario: "certmanager-issuer path only",
			RuleText: "certmanager-issuer/templates/rbac-config.yaml <.Values.global.ingress>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: certmanager-issuer/templates/rbac-config.yaml:1:67: executing \"certmanager-issuer/templates/rbac-config.yaml\" at <.Values.global.ingress>: nil pointer evaluating interface {}.ingress"),
			}},
		}, {
			Scenario: "gitlab-pages path only",
			RuleText: "gitlab-pages/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: gitlab-pages/templates/serviceaccount.yaml:1:57: executing \"gitlab-pages/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "gitlab-shell path only",
			RuleText: "gitlab-shell/templates/traefik-tcp-ingressroute.yaml <.Values.global.ingress.provider>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: gitlab-shell/templates/traefik-tcp-ingressroute.yaml:2:17: executing \"gitlab-shell/templates/traefik-tcp-ingressroute.yaml\" at <.Values.global.ingress.provider>: nil pointer evaluating interface {}.provider"),
			}},
		}, {
			Scenario: "kas path only",
			RuleText: "kas/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: kas/templates/serviceaccount.yaml:1:57: executing \"kas/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "kas path only",
			RuleText: "kas/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: kas/templates/serviceaccount.yaml:1:57: executing \"kas/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "mailroom path only",
			RuleText: "mailroom/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: mailroom/templates/serviceaccount.yaml:1:57: executing \"mailroom/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "migrations path only",
			RuleText: "migrations/templates/job.yaml <include (print $.Template.BasePath \"/_serviceaccountspec.yaml\") .>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: migrations/templates/job.yaml:2:3: executing \"migrations/templates/job.yaml\" at <include (print $.Template.BasePath \"/_serviceaccountspec.yaml\") .>: error calling include: template: migrations/templates/_serviceaccountspec.yaml:1:57: executing \"migrations/templates/_serviceaccountspec.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "praefect path only",
			RuleText: "praefect/templates/statefulset.yaml <.Values.global.image>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: praefect/templates/statefulset.yaml:1:38: executing \"praefect/templates/statefulset.yaml\" at <.Values.global.image>: nil pointer evaluating interface {}.image"),
			}},
		}, {
			Scenario: "sidekiq path only",
			RuleText: "sidekiq/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: sidekiq/templates/serviceaccount.yaml:1:57: executing \"sidekiq/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "spamcheck path only",
			RuleText: "spamcheck/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: spamcheck/templates/serviceaccount.yaml:1:57: executing \"spamcheck/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.serviceAccount"),
			}},
		}, {
			Scenario: "toolbox path only",
			RuleText: "toolbox/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: toolbox/templates/serviceaccount.yaml:1:57: executing \"toolbox/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		}, {
			Scenario: "minio path only",
			RuleText: "minio/templates/pdb.yaml <{{template \"gitlab.pdb.apiVersion\" $pdbCfg}}>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: minio/templates/pdb.yaml:3:24: executing \"minio/templates/pdb.yaml\" at <{{template \"gitlab.pdb.apiVersion\" $pdbCfg}}>: template \"gitlab.pdb.apiVersion\" not defined"),
			}},
		}, {
			Scenario: "nginx-ingress path only",
			RuleText: "nginx-ingress/templates/admission-webhooks/job-patch/serviceaccount.yaml <.Values.admissionWebhooks.serviceAccount.automountServiceAccountToken>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: nginx-ingress/templates/admission-webhooks/job-patch/serviceaccount.yaml:13:40: executing \"nginx-ingress/templates/admission-webhooks/job-patch/serviceaccount.yaml\" at <.Values.admissionWebhooks.serviceAccount.automountServiceAccountToken>: nil pointer evaluating interface {}.serviceAccount"),
			}},
		}, {
			Scenario: "registry path only",
			RuleText: "registry/templates/serviceaccount.yaml <.Values.global.serviceAccount.enabled>",
			Ignorables: []support.Message{{
				Path: "templates/",
				Err:  fmt.Errorf("template: registry/templates/serviceaccount.yaml:1:57: executing \"registry/templates/serviceaccount.yaml\" at <.Values.global.serviceAccount.enabled>: nil pointer evaluating interface {}.enabled"),
			}},
		},

		{
			Scenario: "subchart metadata missing dependencies",
			RuleText: "error_lint_ignore=chart metadata is missing these dependencies**",
			Ignorables: []support.Message{
				{
					Path: "gitlab/chart/charts/gitlab",
					Err:  fmt.Errorf("chart metadata is missing these dependencies: sidekiq,spamcheck,gitaly,gitlab-shell,kas,mailroom,migrations,toolbox,geo-logcursor,gitlab-exporter,webservice"),
				},
			},
		},
		{
			Scenario: "subchart icon is recommended",
			RuleText: "error_lint_ignore=icon is recommended",
			Ignorables: []support.Message{{
				Path: "Chart.yaml",
				Err:  fmt.Errorf("icon is recommended"),
			}},
		},
		{
			Scenario: "subchart values file does not exist",
			RuleText: "error_lint_ignore=file does not exist",
			Ignorables: []support.Message{
				{Path: "values.yaml", Err: fmt.Errorf("file does not exist")},
			},
		},
		{
			Scenario: "Chart.yaml missing apiVersion",
			RuleText: "error_lint_ignore=apiVersion is required. The value must be either \"v1\" or \"v2\"",
			Ignorables: []support.Message{
				{
					Severity: support.ErrorSev,
					Path:     "Chart.yaml",
					Err:      fmt.Errorf("apiVersion is required. The value must be either \"v1\" or \"v2\""),
				},
			},
		},
		{
			Scenario: "values.yaml does not exist",
			RuleText: "error_lint_ignore=file does not exist",
			Ignorables: []support.Message{
				{
					Severity: support.InfoSev,
					Path:     "values.yaml",
					Err:      fmt.Errorf("file does not exist"),
				},
			},
		},
		{
			Scenario: "missing dependencies in chart directory",
			RuleText: "error_lint_ignore=chart directory is missing these dependencies: mariadb",
			Ignorables: []support.Message{
				{
					Severity: support.WarningSev,
					Path:     "/Users/daniel/radius/bb/helm/pkg/action/testdata/charts/chart-missing-deps-but-ignorable",
					Err:      fmt.Errorf("chart directory is missing these dependencies: mariadb"),
				},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.Scenario, func(t *testing.T) {
			matchers := LoadFromReader(strings.NewReader(testCase.RuleText))
			if len(matchers) == 0 {
				t.Fatal("Expected a match, got none.", testCase.Scenario)
			}
			assert.Equal(t, 1, len(matchers))
			matcher := matchers[0]

			for _, ignorableMessage := range testCase.Ignorables {
				got := matcher.Match(ignorableMessage.Err)
				assert.True(t, got, testCase.Scenario)
			}

			keepableMessage := support.NewMessage(support.ErrorSev, "wow/", fmt.Errorf("incredible: something just happened"))
			got := matcher.Match(keepableMessage.Err)
			assert.False(t, got, testCase.Scenario)
		})
	}
}
