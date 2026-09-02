/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// featureGateMetricName is the Prometheus gauge every Kubernetes component
// built on k8s.io/component-base exposes on its own /metrics endpoint,
// recording the enablement state of every feature gate known to that
// process. See:
// https://kubernetes.io/docs/tasks/administer-cluster/configure-feature-gates/#check-via-metrics-endpoint
const featureGateMetricName = "kubernetes_feature_enabled"

// defaultKubeletPort is used when a Node does not report a kubelet port.
const defaultKubeletPort = 10250

// featureGateFetchTimeout bounds each network call in this file: rendering
// happens outside the window helm's --timeout flag covers.
const featureGateFetchTimeout = 10 * time.Second

// supportedKubeFeatureGateComponents are the components this version of Helm
// can actively verify feature gates for. Components accepted by chart
// validation (chart.KubeFeatureGateComponents) but not listed here are only
// ever warned about, never enforced: checking them requires machinery Helm
// does not have today (pod discovery plus a port-forward/exec tunnel), and
// they are unreachable on most managed Kubernetes offerings regardless.
var supportedKubeFeatureGateComponents = map[string]bool{
	"apiserver": true,
	"kubelet":   true,
}

// mergedKubeFeatureGates collects the kubeFeatureGates requirements declared
// by chrt and all of its dependencies (recursively) into a single map, so a
// component is only ever polled once regardless of how many chart levels
// declare requirements for it. Conflicting requirements for the same gate
// from different chart levels are a chart authoring error, not a cluster
// state to warn about, so they fail immediately.
func mergedKubeFeatureGates(chrt *chart.Chart) (map[string]map[string]bool, error) {
	type declaration struct {
		chart string
		want  bool
	}
	declaredBy := map[string]map[string]declaration{}

	var walk func(c *chart.Chart) error
	walk = func(c *chart.Chart) error {
		for component, gates := range c.Metadata.KubeFeatureGates {
			if declaredBy[component] == nil {
				declaredBy[component] = map[string]declaration{}
			}
			for gate, want := range gates {
				if prior, ok := declaredBy[component][gate]; ok && prior.want != want {
					return fmt.Errorf("chart %q requires kubeFeatureGates.%s.%s=%t but chart %q requires %t", c.Name(), component, gate, want, prior.chart, prior.want)
				}
				declaredBy[component][gate] = declaration{chart: c.Name(), want: want}
			}
		}
		for _, dep := range c.Dependencies() {
			if err := walk(dep); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(chrt); err != nil {
		return nil, err
	}

	merged := make(map[string]map[string]bool, len(declaredBy))
	for component, gates := range declaredBy {
		g := make(map[string]bool, len(gates))
		for gate, d := range gates {
			g[gate] = d.want
		}
		merged[component] = g
	}
	return merged, nil
}

// checkKubeFeatureGates verifies that the feature gates required by the
// chart (keyed by component name, then by gate name) match the state
// reported by the cluster. A component missing from wanted is not checked at
// all.
//
// A component this version of Helm does not yet support checking is only
// logged as a warning and does not block installation. For a component Helm
// does support (apiserver, kubelet), any failure to positively confirm a
// required gate -- RBAC denied, component unreachable, timeout, a parse
// error, or the gate simply not being reported -- blocks installation, the
// same as a definitive mismatch.
func (cfg *Configuration) checkKubeFeatureGates(ctx context.Context, wanted map[string]map[string]bool) error {
	components := make([]string, 0, len(wanted))
	for component := range wanted {
		components = append(components, component)
	}
	slices.Sort(components)

	var problems []string
	for _, component := range components {
		gates := wanted[component]
		if len(gates) == 0 {
			continue
		}

		if !supportedKubeFeatureGateComponents[component] {
			cfg.Logger().Warn(
				"chart requires kubeFeatureGates for a component Helm cannot yet verify; skipping",
				slog.String("component", component),
			)
			continue
		}

		actual, err := cfg.fetchComponentFeatureGates(ctx, component)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: could not verify feature gates: %s", component, err))
			continue
		}

		gateNames := make([]string, 0, len(gates))
		for gate := range gates {
			gateNames = append(gateNames, gate)
		}
		slices.Sort(gateNames)

		for _, gate := range gateNames {
			want := gates[gate]
			got, known := actual[gate]
			if !known {
				problems = append(problems, fmt.Sprintf("%s.%s: not reported by the cluster", component, gate))
				continue
			}
			if got != want {
				problems = append(problems, fmt.Sprintf("%s.%s=%t (cluster reports %t)", component, gate, want, got))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("chart requires kubeFeatureGates that could not be satisfied or verified: %s", strings.Join(problems, ", "))
	}
	return nil
}

// fetchComponentFeatureGates retrieves and parses the feature gate state
// reported by the given component's /metrics endpoint.
func (cfg *Configuration) fetchComponentFeatureGates(ctx context.Context, component string) (map[string]bool, error) {
	if cfg.RESTClientGetter == nil {
		return nil, errors.New("no Kubernetes cluster is configured")
	}

	clientset, err := cfg.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	var raw []byte
	switch component {
	case "apiserver":
		apiserverCtx, cancel := context.WithTimeout(ctx, featureGateFetchTimeout)
		defer cancel()
		raw, err = clientset.CoreV1().RESTClient().Get().RequestURI("/metrics").Do(apiserverCtx).Raw()
	case "kubelet":
		raw, err = fetchKubeletMetrics(ctx, clientset)
	default:
		return nil, fmt.Errorf("component %q is not supported", component)
	}
	if err != nil {
		return nil, err
	}

	return parseFeatureGateMetrics(raw)
}

// fetchKubeletMetrics polls the /metrics endpoint of a single representative
// Ready node, via the API server's node proxy subresource. kubelet feature
// gates are technically per-node; Helm treats one Ready node as
// representative of the cluster rather than polling every node.
func fetchKubeletMetrics(ctx context.Context, clientset kubernetes.Interface) ([]byte, error) {
	listCtx, cancelList := context.WithTimeout(ctx, featureGateFetchTimeout)
	defer cancelList()
	nodes, err := clientset.CoreV1().Nodes().List(listCtx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	node, ok := firstReadyNode(nodes.Items)
	if !ok {
		return nil, errors.New("no Ready node found to check kubelet feature gates against")
	}

	port := node.Status.DaemonEndpoints.KubeletEndpoint.Port
	if port == 0 {
		port = defaultKubeletPort
	}

	metricsCtx, cancelMetrics := context.WithTimeout(ctx, featureGateFetchTimeout)
	defer cancelMetrics()
	return clientset.CoreV1().RESTClient().Get().
		Resource("nodes").
		SubResource("proxy").
		Name(fmt.Sprintf("%s:%d", node.Name, port)).
		Suffix("metrics").
		Do(metricsCtx).Raw()
}

// firstReadyNode returns the first node in the list reporting a True Ready
// condition.
func firstReadyNode(nodes []corev1.Node) (corev1.Node, bool) {
	for _, node := range nodes {
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				return node, true
			}
		}
	}
	return corev1.Node{}, false
}

// parseFeatureGateMetrics parses the raw Prometheus text exposition format
// scraped from a component's /metrics endpoint and extracts the enablement
// state of every feature gate it reports.
func parseFeatureGateMetrics(raw []byte) (map[string]bool, error) {
	// kubernetes_feature_enabled and its labels always use the classic
	// Prometheus name character set; expfmt.TextParser requires an explicit
	// scheme (its zero value is invalid) since it added UTF-8 name support.
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parsing metrics: %w", err)
	}

	family, ok := families[featureGateMetricName]
	if !ok {
		return map[string]bool{}, nil
	}

	gates := make(map[string]bool, len(family.Metric))
	for _, m := range family.Metric {
		var name string
		for _, label := range m.Label {
			if label.GetName() == "name" {
				name = label.GetValue()
				break
			}
		}
		if name == "" || m.Gauge == nil {
			continue
		}
		gates[name] = m.Gauge.GetValue() != 0
	}
	return gates, nil
}
