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
package chartutil

import (
	"k8s.io/helm/pkg/aesutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const ENCRYPTED_ANNOTATION_KEY = "encrypted"

// Encrypt Data of templates and values in Chart with cipherKey
func Encrypt(ch *chart.Chart, cipherKey string) (*chart.Chart, error) {
	cipherBytes := []byte(cipherKey)
	// encrypt templates and values
	ch, err := encryptChart(ch, cipherBytes)
	if err != nil {
		return ch, err
	}
	// encrypt dependencies
	for _, c := range ch.Dependencies {
		_, err := encryptChart(c, cipherBytes)
		if err != nil {
			return ch, err
		}
	}
	return ch, nil
}

func encryptChart(ch *chart.Chart, cipherBytes []byte) (*chart.Chart, error) {
	shouldEncrypt := false
	if len(ch.Metadata.Annotations) == 0 {
		shouldEncrypt = true
	} else if v, ok := ch.Metadata.Annotations[ENCRYPTED_ANNOTATION_KEY]; !ok || v != "true" {
		shouldEncrypt = true
	}
	if shouldEncrypt {
		for _, template := range ch.Templates {
			data, err := aesutil.AesEncrypt(template.Data, cipherBytes)
			if err != nil {
				return ch, err
			}
			template.Data = data
		}
		if ch.Values != nil {
			data, err := aesutil.AesEncrypt([]byte(ch.Values.Raw), cipherBytes)
			if err != nil {
				return ch, err
			}
			ch.Values.Raw = string(data)
		}
		if ch.Metadata.Annotations == nil {
			ch.Metadata.Annotations = map[string]string{}
		}
		ch.Metadata.Annotations[ENCRYPTED_ANNOTATION_KEY] = "true"
	}
	return ch, nil
}

// Decrypt Data of templates and values in Chart with cipherKey
func Decrypt(ch *chart.Chart, cipherKey string) (*chart.Chart, error) {
	cipherBytes := []byte(cipherKey)
	// decrypt templates and values
	ch, err := decryptChart(ch, cipherBytes)
	if err != nil {
		return ch, err
	}
	// decrypt dependencies
	for _, c := range ch.Dependencies {
		_, err := decryptChart(c, cipherBytes)
		if err != nil {
			return ch, err
		}
	}
	return ch, nil
}

func decryptChart(ch *chart.Chart, cipherBytes []byte) (*chart.Chart, error) {
	if len(ch.Metadata.Annotations) > 0 {
		if v, ok := ch.Metadata.Annotations[ENCRYPTED_ANNOTATION_KEY]; ok && v == "true" {
			for _, template := range ch.Templates {
				data, err := aesutil.AesDecrypt(template.Data, cipherBytes)
				if err != nil {
					return ch, err
				}
				template.Data = data
			}
			if ch.Values != nil {
				data, err := aesutil.AesDecrypt([]byte(ch.Values.Raw), cipherBytes)
				if err != nil {
					return ch, err
				}
				ch.Values.Raw = string(data)
			}
			ch.Metadata.Annotations[ENCRYPTED_ANNOTATION_KEY] = "false"
		}
	}
	return ch, nil
}
