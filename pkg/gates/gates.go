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

package gates

import (
	"fmt"
	"os"
)

// Gate is the name of the feature gate.
type Gate string

// String returns the string representation of this feature gate.
func (g Gate) String() string {
	return string(g)
}

// IsEnabled determines whether a certain feature gate is enabled.
func (g Gate) IsEnabled() bool {
	return os.Getenv(string(g)) != ""
}

func (g Gate) Error() error {
	return fmt.Errorf("this feature has been marked as experimental and is not enabled by default. Please set %s=1 in your environment to use this feature", g.String())
}
