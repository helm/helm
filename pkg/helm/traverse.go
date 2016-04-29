package helm

import (
	chartutil "github.com/kubernetes/helm/pkg/chart"
)

//
// TODO - we should probably consolidate
// most of the code in this package, that
// is specific to charts, into chartutil.
//

// Walk a chart's dependency tree, returning
// a pointer to the root chart.
//
// The following is an example chart dependency
// hierarchy and the structure of a chartObj
// post traversal. (note some chart files are
// omitted for brevity),
//
// mychart/
//   charts/
//      chart_A/
//        charts/
//           chart_B/
//           chart_C/
//             charts/
//                chart_F/
//       chart_D/
//         charts/
//            chart_E/
//            chart_F/
//
//
// chart: mychart (deps = 2)
//      |
//      |----> chart_A (deps = 2)
//          |
//          |--------> chart_B (deps = 0)
//          |
//          |--------> chart_C (deps = 1)
//              |
//              |------------> chart_F (deps = 0)
//      |
//      |----> chart_D (deps = 2)
//          |
//          |--------> chart_E (deps = 0)
//          |
//          |--------> chart_F (deps = 0)
//
//

func WalkChartFile(chfi *chartutil.Chart) (*chartObj, error) {
	root := &chartObj{file: chfi}
	err := root.walkChartDeps(chfi)

	return root, err
}

type chartObj struct {
	file *chartutil.Chart
	deps []*chartObj
}

func (chd *chartObj) File() *chartutil.Chart {
	return chd.file
}

func (chs *chartObj) walkChartDeps(chfi *chartutil.Chart) error {
	if hasDeps(chfi) {
		names, err := chfi.ChartDepNames()
		if err != nil {
			return err
		}

		if len(names) > 0 {
			chs.deps = append(chs.deps, resolveChartDeps(names)...)
		}
	}

	return nil
}

func resolveChartDeps(names []string) (deps []*chartObj) {
	for _, name := range names {
		chfi, err := chartutil.LoadDir(name)
		if err != nil {
			return
		}

		chs := &chartObj{file: chfi}
		err = chs.walkChartDeps(chfi)
		if err != nil {
			return
		}

		deps = append(deps, chs)
	}

	return
}

func hasDeps(chfi *chartutil.Chart) bool {
	names, err := chfi.ChartDepNames()
	if err != nil {
		return false
	}

	return chfi.ChartsDir() != "" && len(names) > 0
}
