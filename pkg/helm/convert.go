package helm

import (
	"bytes"

	chartutil "k8s.io/helm/pkg/chart"
	chartpbs "k8s.io/helm/pkg/proto/hapi/chart"
)

// ChartToProto converts a chart to its Protobuf struct representation.
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

	chpb.Values, err = ValuesToProto(ch)
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

// MetadataToProto converts Chart.yaml data into  protocol buffere Metadata.
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

// TemplatesToProto converts chart templates to their protobuf representation.
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

// OverridesToProto converts arbitrary TOML override data to Config data.
func OverridesToProto(values []byte) *chartpbs.Config {
	return &chartpbs.Config{
		Raw: string(values),
	}
}

// ValuesToProto converts a chart's values.toml data to protobuf.
func ValuesToProto(ch *chartutil.Chart) (*chartpbs.Config, error) {
	if ch == nil {
		return nil, ErrMissingChart
	}

	vals, err := ch.LoadValues()
	if err != nil {
		//return nil, ErrMissingValues
		vals = map[string]interface{}{}
	}

	var buf bytes.Buffer
	if err = vals.Encode(&buf); err != nil {
		return nil, err
	}

	cfgVals := new(chartpbs.Config)
	cfgVals.Raw = buf.String()

	return cfgVals, nil
}
