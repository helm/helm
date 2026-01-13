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

package logging

import "log/slog"

// LoggerSetterGetter is an interface that can set and get a logger
type LoggerSetterGetter interface {
	// SetLogger sets a new slog.Handler
	SetLogger(newHandler slog.Handler)
	// Logger returns the slog.Logger created from the slog.Handler
	Logger() *slog.Logger
}
