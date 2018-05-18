package lua

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yuin/gopher-lua"

	hapi "k8s.io/helm/pkg/hapi/chart"
)

func TestLoadScripts(t *testing.T) {
	chart := &hapi.Chart{
		Metadata: &hapi.Metadata{
			Name: "starboard",
		},
		Ext: []*hapi.File{
			{
				Name: filepath.Join("ext", "lua", "chart.lua"),
				Data: []byte(`hello="world"; name="Ishmael"`),
			},
			{
				Name: filepath.Join("ext", "lua", "decoy.lua"),
				Data: []byte(`hello="goodbye"`),
			},
		},
	}

	outterChart := &hapi.Chart{
		Metadata: &hapi.Metadata{
			Name: "port",
		},
		Ext: []*hapi.File{
			{
				Name: filepath.Join("ext", "lua", "chart.lua"),
				Data: []byte(`hello="Nantucket"; goodbye ="Spouter"`),
			},
			{
				Name: filepath.Join("ext", "lua", "decoy.lua"),
				Data: []byte(`hello="goodbye"`),
			},
		},
		Dependencies: []*hapi.Chart{chart},
	}

	// Simple test on a single chart
	vm := lua.NewState()
	LoadScripts(vm, chart)

	world := vm.GetGlobal("hello").String()
	assert.Equal(t, world, "world", `expected hello="world"`)

	// Test on a nested chart
	vm = lua.NewState()
	LoadScripts(vm, outterChart)

	// This should override the child chart's value
	result := vm.GetGlobal("hello").String()
	assert.Equal(t, result, "Nantucket", `expected hello="Nantucket"`)

	// This should be unchanged
	result = vm.GetGlobal("goodbye").String()
	assert.Equal(t, result, "Spouter", `expected goodbye="Spouter"`)

	// This should come from the child chart
	result = vm.GetGlobal("name").String()
	assert.Equal(t, result, "Ishmael", `expected name="Ishmael"`)
}
