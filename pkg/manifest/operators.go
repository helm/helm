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

package manifest

// Equalto implements = operator for weights
//
// Weights are equal if both Chart and Manifest attributes are equal
func (mw *Weight) Equalto(other *Weight) bool {
	if mw == nil || other == nil {
		return false
	}

	if mw.Chart == other.Chart && mw.Manifest == other.Manifest {
		return true
	}

	return false
}

// LessThan implements < operator for weights
//
// Precedence is Weight.Chart > Weight.Manifest
func (mw *Weight) LessThan(other *Weight) bool {
	if mw == nil || other == nil {
		return false
	}

	if mw.Chart == other.Chart {
		return mw.Manifest < other.Manifest
	}

	return mw.Chart < other.Chart
}

// GreaterThan implements > operator for weights
//
// Precedence is Weight.Chart > Weight.Manifest
func (mw *Weight) GreaterThan(other *Weight) bool {
	if mw == nil || other == nil {
		return false
	}

	if mw.Chart == other.Chart {
		return mw.Manifest > other.Manifest
	}

	return mw.Chart > other.Chart
}
