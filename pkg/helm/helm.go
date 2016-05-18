package helm

import (
	chartutil "github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/proto/hapi/services"
	"golang.org/x/net/context"
)

// Config defines a gRPC client's configuration.
var Config = &config{
	ServAddr: ":44134",
	Insecure: true,
}

// ListReleases lists the current releases.
func ListReleases(limit int, offset string, sort services.ListSort_SortBy, order services.ListSort_SortOrder, filter string) (*services.ListReleasesResponse, error) {
	c := Config.client()
	if err := c.dial(); err != nil {
		return nil, err
	}
	defer c.Close()

	req := &services.ListReleasesRequest{
		Limit:     int64(limit),
		Offset:    offset,
		SortBy:    sort,
		SortOrder: order,
		Filter:    filter,
	}
	cli, err := c.impl.ListReleases(context.TODO(), req, c.cfg.CallOpts()...)
	if err != nil {
		return nil, err
	}

	return cli.Recv()
}

// GetReleaseStatus returns the given release's status.
func GetReleaseStatus(name string) (*services.GetReleaseStatusResponse, error) {
	c := Config.client()
	if err := c.dial(); err != nil {
		return nil, err
	}
	defer c.Close()

	req := &services.GetReleaseStatusRequest{Name: name}
	return c.impl.GetReleaseStatus(context.TODO(), req, c.cfg.CallOpts()...)
}

// GetReleaseContent returns the configuration for a given release.
func GetReleaseContent(name string) (*services.GetReleaseContentResponse, error) {
	c := Config.client()
	if err := c.dial(); err != nil {
		return nil, err
	}
	defer c.Close()

	req := &services.GetReleaseContentRequest{Name: name}
	return c.impl.GetReleaseContent(context.TODO(), req, c.cfg.CallOpts()...)
}

// UpdateRelease updates a release to a new/different chart.
// TODO: This must take more than just name for an arg.
func UpdateRelease(name string) (*services.UpdateReleaseResponse, error) {
	return nil, ErrNotImplemented
}

// UninstallRelease uninstalls a named release and returns the response.
func UninstallRelease(name string) (*services.UninstallReleaseResponse, error) {
	u := &services.UninstallReleaseRequest{
		Name: name,
	}
	return Config.client().uninstall(u)
}

// InstallRelease installs a new chart and returns the release response.
func InstallRelease(rawVals []byte, chStr string, dryRun bool) (*services.InstallReleaseResponse, error) {
	chfi, err := chartutil.LoadChart(chStr)
	if err != nil {
		return nil, err
	}

	chpb, err := ChartToProto(chfi)
	if err != nil {
		return nil, err
	}

	vals := OverridesToProto(rawVals)

	return Config.client().install(&services.InstallReleaseRequest{
		Chart:  chpb,
		Values: vals,
		DryRun: dryRun,
	})
}
