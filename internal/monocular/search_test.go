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

package monocular

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A search response for phpmyadmin containing 2 results
var searchResult = `{"data":[{"id":"stable/phpmyadmin","type":"chart","attributes":{"name":"phpmyadmin","repo":{"name":"stable","url":"https://charts.helm.sh/stable"},"description":"phpMyAdmin is an mysql administration frontend","home":"https://www.phpmyadmin.net/","keywords":["mariadb","mysql","phpmyadmin"],"maintainers":[{"name":"Bitnami","email":"containers@bitnami.com"}],"sources":["https://github.com/bitnami/bitnami-docker-phpmyadmin"],"icon":""},"links":{"self":"/v1/charts/stable/phpmyadmin"},"relationships":{"latestChartVersion":{"data":{"version":"3.0.0","app_version":"4.9.0-1","created":"2019-08-08T17:57:31.38Z","digest":"119c499251bffd4b06ff0cd5ac98c2ce32231f84899fb4825be6c2d90971c742","urls":["https://charts.helm.sh/stable/phpmyadmin-3.0.0.tgz"],"readme":"/v1/assets/stable/phpmyadmin/versions/3.0.0/README.md","values":"/v1/assets/stable/phpmyadmin/versions/3.0.0/values.yaml"},"links":{"self":"/v1/charts/stable/phpmyadmin/versions/3.0.0"}}}},{"id":"bitnami/phpmyadmin","type":"chart","attributes":{"name":"phpmyadmin","repo":{"name":"bitnami","url":"https://charts.bitnami.com"},"description":"phpMyAdmin is an mysql administration frontend","home":"https://www.phpmyadmin.net/","keywords":["mariadb","mysql","phpmyadmin"],"maintainers":[{"name":"Bitnami","email":"containers@bitnami.com"}],"sources":["https://github.com/bitnami/bitnami-docker-phpmyadmin"],"icon":""},"links":{"self":"/v1/charts/bitnami/phpmyadmin"},"relationships":{"latestChartVersion":{"data":{"version":"3.0.0","app_version":"4.9.0-1","created":"2019-08-08T18:34:13.341Z","digest":"66d77cf6d8c2b52c488d0a294cd4996bd5bad8dc41d3829c394498fb401c008a","urls":["https://charts.bitnami.com/bitnami/phpmyadmin-3.0.0.tgz"],"readme":"/v1/assets/bitnami/phpmyadmin/versions/3.0.0/README.md","values":"/v1/assets/bitnami/phpmyadmin/versions/3.0.0/values.yaml"},"links":{"self":"/v1/charts/bitnami/phpmyadmin/versions/3.0.0"}}}}]}`

func TestSearch(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, searchResult)
	}))
	defer ts.Close()

	c, err := New(ts.URL)
	if err != nil {
		t.Errorf("unable to create monocular client: %s", err)
	}

	results, err := c.Search("phpmyadmin")
	if err != nil {
		t.Errorf("unable to search monocular: %s", err)
	}

	if len(results) != 2 {
		t.Error("Did not receive the expected number of results")
	}
}
