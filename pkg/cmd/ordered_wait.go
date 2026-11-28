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

package cmd

import (
	"fmt"

	chartv3 "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/pkg/chart"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
)

func validateOrderedWaitChart(ws kube.WaitStrategy, ch chart.Charter) error {
	switch ch := ch.(type) {
	case *chartv2.Chart:
		return validateOrderedWaitChartAPIVersion(ws, ch.Metadata.Name, ch.Metadata.APIVersion)
	case *chartv3.Chart:
		return validateOrderedWaitChartAPIVersion(ws, ch.Metadata.Name, ch.Metadata.APIVersion)
	default:
		return nil
	}
}

func validateOrderedWaitChartAPIVersion(ws kube.WaitStrategy, chartName, apiVersion string) error {
	if ws == kube.OrderedWaitStrategy && apiVersion != chartv3.APIVersionV3 {
		return fmt.Errorf("--wait=ordered requires chart apiVersion v3 (chart %q has apiVersion %s)", chartName, apiVersion)
	}
	return nil
}
