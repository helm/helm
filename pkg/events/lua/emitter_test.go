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

package lua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yuin/gopher-lua"

	"k8s.io/helm/pkg/events"
)

func luaFunc(l *lua.LState) int {
	println("PING")
	return 0
}

var mockCtx = &events.Context{}

func TestEmitter_onWrapper(t *testing.T) {
	emitter := events.New()
	vm := lua.NewState()
	defer vm.Close()

	lee := &Emitter{
		parent: emitter,
		vm:     vm,
	}

	lee.onWrapper("ping", vm.NewFunction(luaFunc))

	assert.Nil(t, emitter.Emit("ping", mockCtx), "Expect no errors")
}

const onScript = `
local events = require("events")

events.on("ping", function(c) return "pong" end)
events.on("ping", function(c) return "zoinks" end)
events.on("other thing", function(c) return "hi" end)
`

func TestEmitter_On(t *testing.T) {
	emitter := events.New()
	vm := lua.NewState()
	defer vm.Close()

	lee := New(vm, emitter)
	assert.Nil(t, vm.DoString(onScript), "load script")

	// Emit an event in the Go events system, and expect it to propagate through
	// the Lua system
	lee.parent.Emit("ping", mockCtx)

	// This is a trivial check. We just make sure that the top of the stack is
	// the return value from calling "ping" event
	result := vm.CheckString(1)
	assert.Equal(t, "pong", result, "expect ping event to return pong")
	println(vm.CheckString(2))

}

const luaTableScript = `
example = {
	key = "value",
	number = 123,
	boolean = true,
	nada = nil,
	inner = {
		name = "Matt"
	},
	list = { "one", "two" },
}
`

func TestTableToMap(t *testing.T) {
	vm := lua.NewState()
	vm.DoString(luaTableScript)
	table := vm.GetGlobal("example")
	res := tableToMap(table.(*lua.LTable))

	if res["key"] != "value" {
		t.Error("Expected key to be value")
	}

	if res["number"] != float64(123) {
		t.Errorf("Expected number to be a float64 123, got %+v", res["number"])
	}

	if res["boolean"] != true {
		t.Errorf("Expected bool true")
	}

	if res["nada"] != nil {
		t.Error("Expected nada to be nil")
	}

	if res["inner"].(map[string]interface{})["name"] != "Matt" {
		t.Error("Expected inner.name to be Matt")
	}

	list := []string{"one", "two"}
	for i, val := range res["list"].([]interface{}) {
		if val != list[i] {
			t.Error("Expected list item to be", list[i])
		}
	}

	t.Logf("res: %+v", res)
}
