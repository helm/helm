package lua

import (
	"fmt"
	"path/filepath"

	"github.com/yuin/gopher-lua"

	hapi "k8s.io/helm/pkg/hapi/chart"
)

// LoadScripts is a depth-first script loader for Lua scripts
//
// It walks the chart and all dependencies, loading the ext/lua/chart.lua
// script for each chart.
//
// If a script fails to load, loading immediately ceases and the error is returned.
func LoadScripts(vm *lua.LState, chart *hapi.Chart) error {
	// We go depth first so that the top level chart gets the final word.
	// That is, the top level chart should be able to modify objects that the
	// child charts set.
	for _, child := range chart.Dependencies {
		LoadScripts(vm, child)
	}
	// For now, we only load a `chart.lua`, since that is how other Lua impls
	// do it (e.g. single entrypoint, not multi).
	for _, script := range chart.Ext {
		target := filepath.Join("ext", "lua", "chart.lua")
		if script.Name == target {
			if err := vm.DoString(string(script.Data)); err != nil {
				return fmt.Errorf("failed to execute Lua for %s: %s", chart.Metadata.Name, err)
			}
		}
	}
	return nil
}
