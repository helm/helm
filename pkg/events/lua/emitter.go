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
	"github.com/yuin/gopher-lua"

	"k8s.io/helm/pkg/events"

	"layeh.com/gopher-luar"

	"gopkg.in/yaml.v2"
)

// Emitter is a Lua-specific wrapper around an event.Emitter
//
// This wrapper servces two purposes.
//
// First, it exposes the events API to Lua code. An 'events' module is created,
// and methods are attached to it.
//
// Second, it transforms Lua events back into Go datastructures.
//
// When New() creates a new Emitter, it registers the events library within the
// given VM. This means that you should not try to create multiple emitters for
// a single Lua VM.
//
// Inside of the Lua VM, scripts can call `events.on`:
//
//	-- This is Lua code
//	local events = require("events")
//	events.on("some_event", some_callback)
//
// When an event is triggered on the paremt events.Emitter, this Emitter will
// cause that event to trigger within the Lua VM.
//
// Note that in this model, Go events, Lua events, and events from other systems
// that implement this system can all interoperate with each other. However, the
// only shared pieces of context are (appropriately) the events.Context object
// and whatever Emitter methods are exposed.
type Emitter struct {
	parent *events.Emitter
	vm     *lua.LState
}

// Bind links the vm and the events.Emitter via a lua.Emitter that is not returned.
func Bind(vm *lua.LState, emitter *events.Emitter) {
	New(vm, emitter)
}

// New creates a new *lua.Emitter
//
// It registers the 'events' module into the Lua engine as well. If you do
// not want the events module registered (e.g. if you have done so already, or
// are doing it manually), you may construct a raw Emitter.
func New(vm *lua.LState, emitter *events.Emitter) *Emitter {
	lee := &Emitter{
		parent: emitter,
		vm:     vm,
	}
	// Register handlers inside the VM
	lee.register()
	return lee
}

// register creates a Lua module named "events" and registers the event handlers.
func (l *Emitter) register() {

	exports := map[string]lua.LGFunction{
		"on": l.On,
	}

	l.vm.PreloadModule("events", func(vm *lua.LState) int {
		module := vm.SetFuncs(vm.NewTable(), exports)
		vm.Push(module)
		return 1
	})

	// YAML module
	yaml := map[string]lua.LGFunction{
		"encode": func(vm *lua.LState) int {
			input := vm.CheckTable(1)
			data, err := yaml.Marshal(tableToMap(input))
			if err != nil {
				panic(err)
			}
			vm.Push(lua.LString(string(data)))
			return 1
		},
		"decode": func(vm *lua.LState) int {
			data := vm.CheckString(1)
			dest := map[string]interface{}{}
			if err := yaml.Unmarshal([]byte(data), dest); err != nil {
				panic(err)
			}
			return 0
		},
	}
	l.vm.PreloadModule("yaml", func(vm *lua.LState) int {
		module := vm.SetFuncs(vm.NewTable(), yaml)
		vm.Push(module)
		return 1
	})
}

// On is the main event handler registration function.
//
// The given LState must be a function call with (eventName, callbackFunction) as
// the signature.
func (l *Emitter) On(vm *lua.LState) int {
	eventName := vm.CheckString(1)
	fn := vm.CheckFunction(2)
	l.onWrapper(eventName, fn)
	return 1
}

// onWrapper registers an event handler with the parent events.Emitter.
//
// A wrapper function is applied to match Lua's types to the expected Go types.
// Likewise the events.Context object is exposed in Lua as a UserData object.
func (l *Emitter) onWrapper(event string, fn *lua.LFunction) {
	l.parent.On(event, func(ctx *events.Context) error {
		println("called wrapper for", event)

		lctx := luar.New(l.vm, ctx)

		/*
			luaCtx := l.vm.NewUserData()
			luaCtx.Value = ctx
			l.vm.SetMetatable(luaCtx, l.vm.GetTypeMetatable("context"))
		*/

		// I think we may need to trap a panic here.
		return l.vm.CallByParam(lua.P{
			Fn:      fn,
			NRet:    1,
			Protect: true,
		}, lctx)
	})
}

func tableToMap(table *lua.LTable) map[string]interface{} {
	res := map[string]interface{}{}
	table.ForEach(func(k, v lua.LValue) {
		key := ""
		switch k.Type() {
		case lua.LTString, lua.LTNumber:
			key = k.String()
		default:
			panic("cannot convert table key to string")
		}

		switch raw := v.(type) {
		case lua.LString:
			res[key] = string(raw)
		case lua.LBool:
			res[key] = bool(raw)
		case lua.LNumber:
			res[key] = float64(raw)
		case *lua.LUserData:
			res[key] = raw.Value
		case *lua.LTable:
			// Test whether slice or map
			intkeys := true
			raw.ForEach(func(k, v lua.LValue) {
				if k.Type() != lua.LTNumber {
					intkeys = false
				}
			})
			if intkeys {
				res[key] = tableToSlice(raw)
				return
			}
			res[key] = tableToMap(raw)
		default:
			if raw == lua.LNil {
				res[key] = nil
				return
			}
			panic("unknown")
		}
	})
	return res
}

func tableToSlice(numberTable *lua.LTable) []interface{} {
	slice := []interface{}{}
	numberTable.ForEach(func(k, v lua.LValue) {
		switch raw := v.(type) {
		case lua.LString:
			slice = append(slice, string(raw))
		case lua.LBool:
			slice = append(slice, bool(raw))
		case lua.LNumber:
			slice = append(slice, float64(raw))
		case *lua.LUserData:
			slice = append(slice, raw.Value)
		case *lua.LTable:
			// Test whether slice or map
			intkeys := true
			raw.ForEach(func(k, v lua.LValue) {
				if k.Type() != lua.LTNumber {
					intkeys = false
				}
			})
			if intkeys {
				slice = append(slice, tableToSlice(raw))
				return
			}
			slice = append(slice, tableToMap(raw))
		default:
			if raw == lua.LNil {
				slice = append(slice, nil)
				return
			}
			panic("unknown")
		}
	})
	return slice
}
