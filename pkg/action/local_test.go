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

package action

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandLocalPath(t *testing.T) {
	req := require.New(t)

	glob, err := expandFilePath("testdata/output/*.txt")
	req.NoError(err)
	req.Contains(glob, "testdata/output/list-compressed-deps.txt")
	req.Contains(glob, "testdata/output/list-missing-deps.txt")

	dir, err := expandFilePath("testdata/files/")
	req.NoError(err)
	req.Contains(dir, "testdata/files/external.txt")

	_, err = expandFilePath("testdata/non_existing")
	req.Error(err)
}
