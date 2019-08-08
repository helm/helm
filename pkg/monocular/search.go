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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"helm.sh/helm/pkg/chart"
)

// The structs below represent the structure of the response from the monocular
// search API.

// SearchResult represents an individual chart result
type SearchResult struct {
	ID            string        `json:"id"`
	Type          string        `json:"type"`
	Attributes    Attributes    `json:"attributes"`
	Links         Links         `json:"links"`
	Relationships Relationships `json:"relationships"`
}

// Attributes is the attributes for the chart
type Attributes struct {
	Name        string             `json:"name"`
	Repo        Repo               `json:"repo"`
	Description string             `json:"description"`
	Home        string             `json:"home"`
	Keywords    []string           `json:"keywords"`
	Maintainers []chart.Maintainer `json:"maintainers"`
	Sources     []string           `json:"sources"`
	Icon        string             `json:"icon"`
}

// Repo contains the name in monocular the the url for the repository
type Repo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Links provides a set of links relative to the chartsvc base
type Links struct {
	Self string `json:"self"`
}

// Relationships provides information on the latest version of the chart
type Relationships struct {
	LatestChartVersion LatestChartVersion `json:"latestChartVersion"`
}

// LatestChartVersion provides the details on the latest version of the chart
type LatestChartVersion struct {
	Data  Data  `json:"data"`
	Links Links `json:"links"`
}

// Data provides the specific data on the chart version
type Data struct {
	Version    string    `json:"version"`
	AppVersion string    `json:"app_version"`
	Created    time.Time `json:"created"`
	Digest     string    `json:"digest"`
	Urls       []string  `json:"urls"`
	Readme     string    `json:"readme"`
	Values     string    `json:"values"`
}

// Search performs a search against the monocular search API
func (c *Client) Search(term string) ([]SearchResult, error) {

	// Create the URL to the search endpoint
	// Note, this is currently an internal API for the Hub. This should be
	// formatted without showing how monocular operates.
	p, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}

	// Set the path to the monocular API endpoint for search
	p.Path = path.Join(p.Path, "api/chartsvc/v1/charts/search")

	p.RawQuery = "q=" + url.QueryEscape(term)

	// Create request
	req, err := http.NewRequest("GET", p.String(), nil)
	if err != nil {
		return nil, err
	}

	// Set the user agent so that monocular can identify where the request
	// is coming from
	req.Header.Set("User-Agent", c.UserAgent)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch %s : %s", p.String(), res.Status)
	}

	result := &searchResponse{}

	json.NewDecoder(res.Body).Decode(result)

	return result.Data, nil
}

type searchResponse struct {
	Data []SearchResult `json:"data"`
}
