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
