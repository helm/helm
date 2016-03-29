/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package expansion

import (
	"github.com/kubernetes/helm/pkg/chart"

	"fmt"
)

// ValidateRequest does basic sanity checks on the request.
func ValidateRequest(request *ServiceRequest) error {
	if request.ChartInvocation == nil {
		return fmt.Errorf("Request does not have invocation field")
	}
	if request.Chart == nil {
		return fmt.Errorf("Request does not have chart field")
	}

	chartInv := request.ChartInvocation
	chartFile := request.Chart.Chartfile

	l, err := chart.Parse(chartInv.Type)
	if err != nil {
		return fmt.Errorf("cannot parse chart reference %s: %s", chartInv.Type, err)
	}

	if l.Name != chartFile.Name {
		return fmt.Errorf("Chart invocation type (%s) does not match provided chart (%s)", chartInv.Type, chartFile.Name)
	}

	if chartFile.Expander == nil {
		message := fmt.Sprintf("Chart JSON does not have expander field")
		return fmt.Errorf("%s: %s", chartInv.Name, message)
	}

	return nil
}
