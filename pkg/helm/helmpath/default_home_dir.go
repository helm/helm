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

package helmpath

import (
	"fmt"
	"github.com/casimir/xdg-go"
	"io"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

// Old default helm home, it's old good ~/.helm
var oldDefaultHelmHome = filepath.Join(homedir.HomeDir(), ".helm")

// New default helm home, with different paths for different OS:
//	- %APPDATA%\helm on Windows
//	- ~/Library/Preferences/helm on OSX
//  - $XDG_CONFIG_DIR/helm (typically ~/.config/helm for linux)
var defaultHelmHome = filepath.Join(xdg.ConfigHome(), "helm")

func DirExists(path string) bool {
	osStat, err := os.Stat(path)
	return err == nil && osStat.IsDir()
}

// Check whether new default helm home exists
// TODO: improve me
var DefaultHelmHomeExists = func() bool {
	return DirExists(defaultHelmHome)
}

// Checks whether old-style ~/.helm exists
// TODO: improve me
var OldDefaultHelmHomeExists = func() bool {
	return DirExists(oldDefaultHelmHome)
}

// Get configuration home dir.
//
// Note: Temporal until all migrate to XDG Base Directory spec
func GetDefaultConfigHome(out io.Writer) string {
	if DefaultHelmHomeExists() || !OldDefaultHelmHomeExists() {
		return defaultHelmHome
	}
	fmt.Fprintf(out, "WARNING: using old-style configuration directory. Please, consider moving it to %s\n", defaultHelmHome)
	return oldDefaultHelmHome
}
