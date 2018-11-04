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

package require

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NoArgs returns an error if any args are included.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errors.Errorf(
			"%q accepts no arguments\n\nUsage:  %s",
			cmd.CommandPath(),
			cmd.UseLine(),
		)
	}
	return nil
}

// ExactArgs returns an error if there are not exactly n args.
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return errors.Errorf(
				"%q requires %d %s\n\nUsage:  %s",
				cmd.CommandPath(),
				n,
				pluralize("argument", n),
				cmd.UseLine(),
			)
		}
		return nil
	}
}

// MaximumNArgs returns an error if there are more than N args.
func MaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return errors.Errorf(
				"%q accepts at most %d %s\n\nUsage:  %s",
				cmd.CommandPath(),
				n,
				pluralize("argument", n),
				cmd.UseLine(),
			)
		}
		return nil
	}
}

// MinimumNArgs returns an error if there is not at least N args.
func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return errors.Errorf(
				"%q requires at least %d %s\n\nUsage:  %s",
				cmd.CommandPath(),
				n,
				pluralize("argument", n),
				cmd.UseLine(),
			)
		}
		return nil
	}
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
