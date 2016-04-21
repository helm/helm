package helm

import (
	"github.com/deis/tiller/pkg/chart"
	chartpb "github.com/deis/tiller/pkg/proto/hapi/chart"
	"github.com/deis/tiller/pkg/proto/hapi/services"
)

var Config = &config{
	ServAddr: ":44134",
	Insecure: true,
}

func ListReleases(limit, offset int) (<-chan *services.ListReleasesResponse, error) {
	return nil, errNotImplemented
}

func GetReleaseStatus(name string) (*services.GetReleaseStatusResponse, error) {
	return nil, errNotImplemented
}

func GetReleaseContent(name string) (*services.GetReleaseContentResponse, error) {
	return nil, errNotImplemented
}

func UpdateRelease(name string) (*services.UpdateReleaseResponse, error) {
	return nil, errNotImplemented
}

func UninstallRelease(name string) (*services.UninstallReleaseResponse, error) {
	return nil, errNotImplemented
}

func InstallRelease(ch *chart.Chart) (res *services.InstallReleaseResponse, err error) {
	chpb := new(chartpb.Chart)

	chpb.Metadata, err = mkProtoMetadata(ch.Chartfile())
	if err != nil {
		return
	}

	chpb.Templates, err = mkProtoTemplates(ch)
	if err != nil {
		return
	}

	chpb.Dependencies, err = mkProtoChartDeps(ch)
	if err != nil {
		return
	}

	var vals *chartpb.Config

	vals, err = mkProtoConfigValues(ch)
	if err != nil {
		return
	}

	res, err = Config.client().install(&services.InstallReleaseRequest{
		Chart:  chpb,
		Values: vals,
	})

	return
}

// pkg/chart to proto/hapi/chart helpers. temporary.
func mkProtoMetadata(ch *chart.Chartfile) (*chartpb.Metadata, error) {
	if ch == nil {
		return nil, errMissingChart
	}

	md := &chartpb.Metadata{
		Name:        ch.Name,
		Home:        ch.Home,
		Version:     ch.Version,
		Description: ch.Description,
	}

	md.Sources = make([]string, len(ch.Source))
	copy(md.Sources, ch.Source)

	md.Keywords = make([]string, len(ch.Keywords))
	copy(md.Keywords, ch.Keywords)

	for _, maintainer := range ch.Maintainers {
		md.Maintainers = append(md.Maintainers, &chartpb.Maintainer{
			Name:  maintainer.Name,
			Email: maintainer.Email,
		})
	}

	return md, nil
}

func mkProtoTemplates(ch *chart.Chart) ([]*chartpb.Template, error) {
	tpls, err := ch.LoadTemplates()
	if err != nil {
		return nil, err
	}

	_ = tpls

	return nil, nil
}

func mkProtoChartDeps(ch *chart.Chart) ([]*chartpb.Chart, error) {
	return nil, nil
}

func mkProtoConfigValues(ch *chart.Chart) (*chartpb.Config, error) {
	vals, err := ch.LoadValues()
	if err != nil {
		return nil, errMissingValues
	}

	_ = vals

	return nil, nil
}
