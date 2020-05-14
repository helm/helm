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

package provenance

import (
	"os"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func FuzzNewFromFiles(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		keyFileBytes, err := ff.GetBytes()
		if err != nil {
			return
		}
		keyFile, err := os.Create("keyFile")
		if err != nil {
			return
		}
		defer keyFile.Close()
		defer os.Remove(keyFile.Name())
		_, err = keyFile.Write(keyFileBytes)
		if err != nil {
			return
		}
		keyringFile, err := os.Create("keyringFile ")
		if err != nil {
			return
		}
		defer keyringFile.Close()
		defer os.Remove(keyringFile.Name())
		keyringFileBytes, err := ff.GetBytes()
		if err != nil {
			return
		}
		_, err = keyringFile.Write(keyringFileBytes)
		if err != nil {
			return
		}
		_, _ = NewFromFiles(keyFile.Name(), keyringFile.Name())
	})
}

func FuzzParseMessageBlock(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = parseMessageBlock(data)
	})
}

func FuzzMessageBlock(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		err := os.Mkdir("fuzzDir", 0755)
		if err != nil {
			return
		}
		defer os.RemoveAll("fuzzDir")
		err = ff.CreateFiles("fuzzDir")
		if err != nil {
			return
		}
		_, _ = messageBlock("fuzzDir")
		return
	})
}
