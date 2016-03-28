/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package main

import (
	"testing"
)

func TestHasValidPrefix(t *testing.T) {
	tests := map[string]bool{
		"https://host/bucket": true,
		"http://host/bucket":  true,
		"gs://bucket-name":    true,
		"charts":              false,
	}

	for url, expectedResult := range tests {
		result := IsValidURL(url)
		if result != expectedResult {
			t.Errorf("Expected: %v, Got: %v for url: %v", expectedResult, result, url)
		}
	}
}
