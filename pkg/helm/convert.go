package helm

import (
	chartutil "github.com/kubernetes/helm/pkg/chart"
	chartpbs "github.com/kubernetes/helm/pkg/proto/hapi/chart"
)

func ChartToProto(ch *chartutil.Chart) (chpb *chartpbs.Chart, err error) {
	chpb = new(chartpbs.Chart)

	chpb.Metadata, err = MetadataToProto(ch)
	if err != nil {
		return
	}

	chpb.Templates, err = TemplatesToProto(ch)
	if err != nil {
		return
	}

	chs, err := WalkChartFile(ch)
	if err != nil {
		return
	}

	for _, dep := range chs.deps {
		chdep, err := ChartToProto(dep.File())
		if err != nil {
			return nil, err
		}

		chpb.Dependencies = append(chpb.Dependencies, chdep)
	}

	return
}

func MetadataToProto(ch *chartutil.Chart) (*chartpbs.Metadata, error) {
	if ch == nil {
		return nil, ErrMissingChart
	}

	chfi := ch.Chartfile()

	md := &chartpbs.Metadata{
		Name:        chfi.Name,
		Home:        chfi.Home,
		Version:     chfi.Version,
		Description: chfi.Description,
	}

	md.Sources = make([]string, len(chfi.Source))
	copy(md.Sources, chfi.Source)

	md.Keywords = make([]string, len(chfi.Keywords))
	copy(md.Keywords, chfi.Keywords)

	for _, maintainer := range chfi.Maintainers {
		md.Maintainers = append(md.Maintainers, &chartpbs.Maintainer{
			Name:  maintainer.Name,
			Email: maintainer.Email,
		})
	}

	return md, nil
}

func TemplatesToProto(ch *chartutil.Chart) (tpls []*chartpbs.Template, err error) {
	if ch == nil {
		return nil, ErrMissingChart
	}

	members, err := ch.LoadTemplates()
	if err != nil {
		return
	}

	var tpl *chartpbs.Template

	for _, member := range members {
		tpl = &chartpbs.Template{
			Name: member.Path,
			Data: make([]byte, len(member.Content)),
		}

		copy(tpl.Data, member.Content)

		tpls = append(tpls, tpl)
	}

	return
}

func ValuesToProto(ch *chartutil.Chart) (*chartpbs.Config, error) {
	vals, err := ch.LoadValues()
	if err != nil {
		return nil, ErrMissingValues
	}

	_ = vals

	return nil, nil
}
