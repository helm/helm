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

package getter

import (
	"bytes"
	"fmt"
)

// Getter is an interface to support GET to the specified URL.
type Getter interface {
	//Get file content by url string
	Get(url string) (*bytes.Buffer, error)
}

//Schemes is the list to represent a specific Getter's protocol capabilities
type Schemes []string

//Constructor is the function for every getter which creates a specific instance
//according to the configuration
type Constructor func(URL, CertFile, KeyFile, CAFile string) (Getter, error)

//Prop represents any getter and its capability
type Prop struct {
	Schemes     Schemes
	Constructor Constructor
}

//ConstructorByScheme returns a contstructor based on the required scheme
func ConstructorByScheme(props []Prop, requiredScheme string) (Constructor, error) {
	for _, item := range props {
		for _, itemScheme := range item.Schemes {
			if itemScheme == requiredScheme {
				return item.Constructor, nil
			}
		}
	}
	return nil, fmt.Errorf("Getter not found")
}
