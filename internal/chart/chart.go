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
package chart

// Chart provides an interface between multiple types of charts.
//
// Chat provides a union between different Chart versions so that functions and generics can work
// with one type. Note, chart v2 handles apiVersion v1 and v2 charts.
// TODO(mattfarina): Add chart v3 here.
type Chart interface{}
