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
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cmd/require"
	release "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

var historyHelp = `
History prints historical revisions for a given release.

A default maximum of 256 revisions will be returned. Setting '--max'
configures the maximum length of the revision list returned.

The historical release set is printed as a formatted table, e.g:

    $ helm history angry-bird
    REVISION    UPDATED                     STATUS          CHART             APP VERSION     DESCRIPTION
    1           Mon Oct 3 10:15:13 2016     superseded      alpine-0.1.0      1.0             Initial install
    2           Mon Oct 3 10:15:13 2016     superseded      alpine-0.1.0      1.0             Upgraded successfully
    3           Mon Oct 3 10:15:13 2016     superseded      alpine-0.1.0      1.0             Rolled back to 2
    4           Mon Oct 3 10:15:13 2016     deployed        alpine-0.1.0      1.0             Upgraded successfully
`

func newHistoryCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewHistory(cfg)
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:     "history RELEASE_NAME",
		Long:    historyHelp,
		Short:   "fetch release history",
		Aliases: []string{"hist"},
		Args:    require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return noMoreArgsComp()
			}
			return compListReleases(toComplete, args, cfg)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			history, err := getHistory(client, args[0])
			if err != nil {
				return err
			}

			return outfmt.Write(out, history)
		},
	}

	f := cmd.Flags()
	f.IntVar(&client.Max, "max", 256, "maximum number of revision to include in history")
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

type releaseInfo struct {
	Revision    int       `json:"revision"`
	Updated     time.Time `json:"updated,omitzero"`
	Status      string    `json:"status"`
	Chart       string    `json:"chart"`
	AppVersion  string    `json:"app_version"`
	Description string    `json:"description"`
}

// releaseInfoJSON is used for custom JSON marshaling/unmarshaling
type releaseInfoJSON struct {
	Revision    int        `json:"revision"`
	Updated     *time.Time `json:"updated,omitempty"`
	Status      string     `json:"status"`
	Chart       string     `json:"chart"`
	AppVersion  string     `json:"app_version"`
	Description string     `json:"description"`
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It handles empty string time fields by treating them as zero values.
func (r *releaseInfo) UnmarshalJSON(data []byte) error {
	// First try to unmarshal into a map to handle empty string time fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Replace empty string time fields with nil
	if val, ok := raw["updated"]; ok {
		if str, ok := val.(string); ok && str == "" {
			raw["updated"] = nil
		}
	}

	// Re-marshal with cleaned data
	cleaned, err := json.Marshal(raw)
	if err != nil {
		return err
	}

	// Unmarshal into temporary struct with pointer time field
	var tmp releaseInfoJSON
	if err := json.Unmarshal(cleaned, &tmp); err != nil {
		return err
	}

	// Copy values to releaseInfo struct
	r.Revision = tmp.Revision
	if tmp.Updated != nil {
		r.Updated = *tmp.Updated
	}
	r.Status = tmp.Status
	r.Chart = tmp.Chart
	r.AppVersion = tmp.AppVersion
	r.Description = tmp.Description

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
// It omits zero-value time fields from the JSON output.
func (r releaseInfo) MarshalJSON() ([]byte, error) {
	tmp := releaseInfoJSON{
		Revision:    r.Revision,
		Status:      r.Status,
		Chart:       r.Chart,
		AppVersion:  r.AppVersion,
		Description: r.Description,
	}

	if !r.Updated.IsZero() {
		tmp.Updated = &r.Updated
	}

	return json.Marshal(tmp)
}

type releaseHistory []releaseInfo

func (r releaseHistory) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, r)
}

func (r releaseHistory) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, r)
}

func (r releaseHistory) WriteTable(out io.Writer) error {
	tbl := uitable.New()
	tbl.AddRow("REVISION", "UPDATED", "STATUS", "CHART", "APP VERSION", "DESCRIPTION")
	for _, item := range r {
		tbl.AddRow(item.Revision, item.Updated.Format(time.ANSIC), item.Status, item.Chart, item.AppVersion, item.Description)
	}
	return output.EncodeTable(out, tbl)
}

func getHistory(client *action.History, name string) (releaseHistory, error) {
	histi, err := client.Run(name)
	if err != nil {
		return nil, err
	}
	hist, err := releaseListToV1List(histi)
	if err != nil {
		return nil, err
	}

	releaseutil.Reverse(hist, releaseutil.SortByRevision)

	var rels []*release.Release
	for i := 0; i < min(len(hist), client.Max); i++ {
		rels = append(rels, hist[i])
	}

	if len(rels) == 0 {
		return releaseHistory{}, nil
	}

	releaseHistory := getReleaseHistory(rels)

	return releaseHistory, nil
}

func getReleaseHistory(rls []*release.Release) (history releaseHistory) {
	for i := len(rls) - 1; i >= 0; i-- {
		r := rls[i]
		c := formatChartName(r.Chart)
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
			rInfo.Updated = r.Info.LastDeployed

		}
		history = append(history, rInfo)
	}

	return history
}

func formatChartName(c *chart.Chart) string {
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

func compListRevisions(_ string, cfg *action.Configuration, releaseName string) ([]string, cobra.ShellCompDirective) {
	client := action.NewHistory(cfg)

	var revisions []string
	if histi, err := client.Run(releaseName); err == nil {
		hist, err := releaseListToV1List(histi)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		for _, version := range hist {
			appVersion := fmt.Sprintf("App: %s", version.Chart.Metadata.AppVersion)
			chartDesc := fmt.Sprintf("Chart: %s-%s", version.Chart.Metadata.Name, version.Chart.Metadata.Version)
			revisions = append(revisions, fmt.Sprintf("%s\t%s, %s", strconv.Itoa(version.Version), appVersion, chartDesc))
		}
		return revisions, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveError
}
