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

// Package lua provides Lua event handling bindings
//
// This package takes the Helm Go event system and extends it on into Lua. The
// library handles the bi-directional transformation of Lua and Go objects.
//
// A major design goal of this implementation is that it will be able to interoperate
// with handlers registered directly in Go and (in the future) other languages
// that are added to the events system. To this end, there are a number of "round
// trips" from Lua to Go and back again that otherwise could have been optimized
// out.
package lua
