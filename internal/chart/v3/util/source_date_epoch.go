package util

import (
	"strconv"
	"time"

	chart "helm.sh/helm/v4/internal/chart/v3"
)

// ParseSourceDateEpochValue parses SOURCE_DATE_EPOCH per https://reproducible-builds.org/docs/source-date-epoch/.
func ParseSourceDateEpochValue(epochStr string) (time.Time, error) {
	epoch, err := strconv.ParseInt(epochStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	if epoch < 0 {
		return time.Time{}, strconv.ErrRange
	}
	return time.Unix(epoch, 0).UTC(), nil
}

// ApplySourceDateEpoch sets timestamps on the chart (and dependencies) to epoch.
func ApplySourceDateEpoch(c *chart.Chart, epoch time.Time) {
	applySourceDateEpoch(c, epoch)
}

func applySourceDateEpoch(c *chart.Chart, epoch time.Time) {
	c.ModTime = epoch
	if len(c.Schema) > 0 {
		c.SchemaModTime = epoch
	}
	if c.Lock != nil {
		c.Lock.Generated = epoch
	}

	for _, f := range c.Raw {
		if f != nil {
			f.ModTime = epoch
		}
	}
	for _, f := range c.Templates {
		if f != nil {
			f.ModTime = epoch
		}
	}
	for _, f := range c.Files {
		if f != nil {
			f.ModTime = epoch
		}
	}
	for _, dep := range c.Dependencies() {
		applySourceDateEpoch(dep, epoch)
	}
}
