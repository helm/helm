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

package schema

import (
	"fmt"
	"time"
)

// TODO: can we generate these plugin input/output messages?

type GetterOptionsV1 struct {
	URL                   string
	CertFile              string
	KeyFile               string
	CAFile                string
	UNTar                 bool
	InsecureSkipVerifyTLS bool
	PlainHTTP             bool
	AcceptHeader          string
	Username              string
	Password              string
	PassCredentialsAll    bool
	UserAgent             string
	Version               string
	Timeout               time.Duration
}

type InputMessageGetterV1 struct {
	Href     string          `json:"href"`
	Protocol string          `json:"protocol"`
	Options  GetterOptionsV1 `json:"options"`
}

type OutputMessageGetterV1 struct {
	Data []byte `json:"data"`
}

// ConfigGetterV1 represents the configuration for download plugins
type ConfigGetterV1 struct {
	// Protocols are the list of URL schemes supported by this downloader
	Protocols []string `yaml:"protocols"`
}

func (c *ConfigGetterV1) Validate() error {
	if len(c.Protocols) == 0 {
		return fmt.Errorf("getter has no protocols")
	}
	for i, protocol := range c.Protocols {
		if protocol == "" {
			return fmt.Errorf("getter has empty protocol at index %d", i)
		}
	}
	return nil
}
