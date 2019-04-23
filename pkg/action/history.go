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
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/release"
	"helm.sh/helm/pkg/releaseutil"
)

type releaseInfo struct {
	Revision    int    `json:"revision"`
	Updated     string `json:"updated"`
	Status      string `json:"status"`
	Chart       string `json:"chart"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

type releaseHistory []releaseInfo

type OutputFormat string

const (
	Table OutputFormat = "table"
	JSON  OutputFormat = "json"
	YAML  OutputFormat = "yaml"
)

var ErrInvalidFormatType = errors.New("invalid format type")

func (o OutputFormat) String() string {
	return string(o)
}

func ParseOutputFormat(s string) (out OutputFormat, err error) {
	switch s {
	case Table.String():
		out, err = Table, nil
	case JSON.String():
		out, err = JSON, nil
	case YAML.String():
		out, err = YAML, nil
	default:
		out, err = "", ErrInvalidFormatType
	}
	return
}

func (o OutputFormat) MarshalHistory(hist releaseHistory) (byt []byte, err error) {
	switch o {
	case YAML:
		byt, err = yaml.Marshal(hist)
	case JSON:
		byt, err = json.Marshal(hist)
	case Table:
		byt = formatAsTable(hist)
	default:
		err = ErrInvalidFormatType
	}
	return
}

// History is the action for checking the release's ledger.
//
// It provides the implementation of 'helm history'.
type History struct {
	cfg *Configuration

	Max          int
	OutputFormat string
}

// NewHistory creates a new History object with the given configuration.
func NewHistory(cfg *Configuration) *History {
	return &History{
		cfg: cfg,
	}
}

// Run executes 'helm history' against the given release.
func (h *History) Run(name string) (string, error) {
	if err := validateReleaseName(name); err != nil {
		return "", errors.Errorf("getHistory: Release name is invalid: %s", name)
	}

	h.cfg.Log("getting history for release %s", name)
	hist, err := h.cfg.Releases.History(name)
	if err != nil {
		return "", err
	}

	releaseutil.Reverse(hist, releaseutil.SortByRevision)

	var rels []*release.Release
	for i := 0; i < min(len(hist), h.Max); i++ {
		rels = append(rels, hist[i])
	}

	if len(rels) == 0 {
		return "", nil
	}

	releaseHistory := getReleaseHistory(rels)

	outputFormat, err := ParseOutputFormat(h.OutputFormat)
	if err != nil {
		return "", err
	}
	history, formattingError := outputFormat.MarshalHistory(releaseHistory)
	if formattingError != nil {
		return "", formattingError
	}

	return string(history), nil
}

func getReleaseHistory(rls []*release.Release) (history releaseHistory) {
	for i := len(rls) - 1; i >= 0; i-- {
		r := rls[i]
		c := formatChartname(r.Chart)
		s := r.Info.Status.String()
		v := r.Version
		d := r.Info.Description
		a := formatAppVersion(r.Chart)

		rInfo := releaseInfo{
			Revision:    v,
			Status:      s,
			Chart:       c,
			AppVersion:  a,
			Description: d,
		}
		if !r.Info.LastDeployed.IsZero() {
			rInfo.Updated = r.Info.LastDeployed.String()

		}
		history = append(history, rInfo)
	}

	return history
}

func formatAsTable(releases releaseHistory) []byte {
	tbl := uitable.New()

	tbl.AddRow("REVISION", "UPDATED", "STATUS", "CHART", "APP VERSION", "DESCRIPTION")
	for i := 0; i <= len(releases)-1; i++ {
		r := releases[i]
		tbl.AddRow(r.Revision, r.Updated, r.Status, r.Chart, r.AppVersion, r.Description)
	}
	return tbl.Bytes()
}

func formatChartname(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		// This is an edge case that has happened in prod, though we don't
		// know how: https://github.com/helm/helm/issues/1347
		return "MISSING"
	}
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func formatAppVersion(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		// This is an edge case that has happened in prod, though we don't
		// know how: https://github.com/helm/helm/issues/1347
		return "MISSING"
	}
	return c.AppVersion()
}
