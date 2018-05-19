/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

/*

import (
	"reflect"

	"github.com/yuin/gopher-lua"
)

// LuaTagName is the tag to be parsed as a Lua annotation
// This is mutable for nefarious purposes.
var LuaTagName = "lua"

var ErrUnsupportedKind = errors.New("unsupported kind")

func luamatic(name string, v interface{}, vm *lua.Lstate) error {
	val := reflect.Indirect(reflect.ValueOf(v))
	switch val.Kind() {
	// TODO: are reflect.Interface, reflect.Ptr, and reflect.Uintptr okay?
	// TODO: can Complex64/128 be represented by Lua
	// TODO: is there any processing we need to do on maps, slices, and arrays?
	case reflect.UnsafePointer, reflect.Chan, reflect.Invalid:
		return ErrUnsupportedKind
	case reflect.Struct:

		// In this case, we create a UserData with associated metatable
		mt := vm.NewTypeMetatable(name)
		// Next we have to walk the struct and find the fields to add.
		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := gettag(&f)
			iface := val.Field(i).Interface()
			if tag.omit {
				continue
			}

			// Now we have a struct tag on a field, and should make it accessible
			// to Lua. Set value to Nil at this point
		}
		mt.SetField(mt, tag.name, lua.LNil)
		vm.SetGlobal(name, mt)

		ud := vm.NewUserData()
		ud.Value = v
		ud.SetMetatable(ud, vm.GetTypeMetatable(name))
		vm.Push(ud)
		return nil
	default:
		obj.Set(n, v)
		for _, a := range aliases {
			obj.Set(a, v)
		}
		return nil
	}
}

func gettag(field *reflect.StructField) luaTag {
	t := field.Tag.Get(LuaTagName)
	if len(t) == 0 {
		return luaTag{name: field.Name}
	}

	// We preserve the convention used by JSON, YAML, and other tags so that
	// an evil library user can change LuaTagName to something and get rational
	// output from this.
	data := strings.Split(t, ",")
	n := data[0]
	if n == "-" {
		return luaTag{name: field.Name, omit: true}
	}
	ot := luaTag{name: n}
	if len(data) == 1 {
		return ot
	}
	for _, k := range data[1:] {
		switch item := k; {
		case strings.HasPrefix(item, "alias="):
			ot.aliases = append(ot.aliases, strings.TrimPrefix(item, "alias="))
		}
	}
	return ot
}

type luaTag struct {
	name    string
	omit    bool
	aliases []string
}
*/
