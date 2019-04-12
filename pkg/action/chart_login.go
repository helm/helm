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
	"context"
	"fmt"
	"io"

	auth "github.com/deislabs/oras/pkg/auth/docker"
)

// ChartLogin performs a chart login operation.
type ChartLogin struct {
	cfg *Configuration
}

// NewChartLogin creates a new ChartLogin object with the given configuration.
func NewChartLogin(cfg *Configuration) *ChartLogin {
	return &ChartLogin{
		cfg: cfg,
	}
}

// Run executes the chart login operation
func (a *ChartLogin) Run(out io.Writer, host string, username string, password string) error {
	cli, err := auth.NewClient("~/.docker/config")
	if err != nil {
		return err
	}
	if err := cli.Login(context.Background(), host, username, password); err != nil {
		return err
	}
	fmt.Println("Login Succeeded")
	return nil
}
