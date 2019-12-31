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

package completion

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/cli"
)

// ==================================================================================
// The below code supports dynamic shell completion in Go.
// This should ultimately be pushed down into Cobra.
// ==================================================================================

// CompRequestCmd Hidden command to request completion results from the program.
// Used by the shell completion script.
const CompRequestCmd = "__complete"

// Global map allowing to find completion functions for commands.
var validArgsFunctions = map[*cobra.Command]func(cmd *cobra.Command, args []string, toComplete string) ([]string, BashCompDirective){}

// BashCompDirective is a bit map representing the different behaviors the shell
// can be instructed to have once completions have been provided.
type BashCompDirective int

const (
	// BashCompDirectiveError indicates an error occurred and completions should be ignored.
	BashCompDirectiveError BashCompDirective = 1 << iota

	// BashCompDirectiveNoSpace indicates that the shell should not add a space
	// after the completion even if there is a single completion provided.
	BashCompDirectiveNoSpace

	// BashCompDirectiveNoFileComp indicates that the shell should not provide
	// file completion even when no completion is provided.
	// This currently does not work for zsh or bash < 4
	BashCompDirectiveNoFileComp

	// BashCompDirectiveDefault indicates to let the shell perform its default
	// behavior after completions have been provided.
	BashCompDirectiveDefault BashCompDirective = 0
)

// RegisterValidArgsFunc should be called to register a function to provide argument completion for a command
func RegisterValidArgsFunc(cmd *cobra.Command, f func(cmd *cobra.Command, args []string, toComplete string) ([]string, BashCompDirective)) {
	if _, exists := validArgsFunctions[cmd]; exists {
		log.Fatal(fmt.Sprintf("RegisterValidArgsFunc: command '%s' already registered", cmd.Name()))
	}
	validArgsFunctions[cmd] = f
}

var debug = true

// Returns a string listing the different directive enabled in the specified parameter
func (d BashCompDirective) string() string {
	var directives []string
	if d&BashCompDirectiveError != 0 {
		directives = append(directives, "BashCompDirectiveError")
	}
	if d&BashCompDirectiveNoSpace != 0 {
		directives = append(directives, "BashCompDirectiveNoSpace")
	}
	if d&BashCompDirectiveNoFileComp != 0 {
		directives = append(directives, "BashCompDirectiveNoFileComp")
	}
	if len(directives) == 0 {
		directives = append(directives, "BashCompDirectiveDefault")
	}

	if d > BashCompDirectiveError+BashCompDirectiveNoSpace+BashCompDirectiveNoFileComp {
		return fmt.Sprintf("ERROR: unexpected BashCompDirective value: %d", d)
	}
	return strings.Join(directives, ", ")
}

// NewCompleteCmd add a special hidden command that an be used to request completions
func NewCompleteCmd(settings *cli.EnvSettings) *cobra.Command {
	debug = settings.Debug
	return &cobra.Command{
		Use:                   fmt.Sprintf("%s [command-line]", CompRequestCmd),
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
		Args:                  require.MinimumNArgs(2),
		Short:                 "Request shell completion choices for the specified command-line",
		Long: fmt.Sprintf("%s is a special command that is used by the shell completion logic\n%s",
			CompRequestCmd, "to request completion choices for the specified command-line."),
		Run: func(cmd *cobra.Command, args []string) {
			CompDebugln(fmt.Sprintf("%s was called with args %v", cmd.Name(), args))

			trimmedArgs := args[:len(args)-1]
			toComplete := args[len(args)-1]

			// Find the real command for which completion must be performed
			finalCmd, finalArgs, err := cmd.Root().Find(trimmedArgs)
			if err != nil {
				// Unable to find the real command. E.g., helm invalidCmd <TAB>
				os.Exit(int(BashCompDirectiveError))
			}

			CompDebugln(fmt.Sprintf("Found final command '%s', with finalArgs %v", finalCmd.Name(), finalArgs))

			// Parse the flags and extract the arguments to prepare for calling the completion function
			if err = finalCmd.ParseFlags(finalArgs); err != nil {
				CompErrorln(fmt.Sprintf("Error while parsing flags from args %v: %s", finalArgs, err.Error()))
				return
			}
			argsWoFlags := finalCmd.Flags().Args()
			CompDebugln(fmt.Sprintf("Args without flags are '%v' with length %d", argsWoFlags, len(argsWoFlags)))

			// Find completion function for the command
			completionFn, ok := validArgsFunctions[finalCmd]
			if !ok {
				CompErrorln(fmt.Sprintf("Dynamic completion not supported/needed for flag or command: %s", finalCmd.Name()))
				return
			}

			CompDebugln(fmt.Sprintf("Calling completion method for subcommand '%s' with args '%v' and toComplete '%s'", finalCmd.Name(), argsWoFlags, toComplete))
			completions, directive := completionFn(finalCmd, argsWoFlags, toComplete)
			for _, comp := range completions {
				// Print each possible completion to stdout for the completion script to consume.
				fmt.Println(comp)
			}

			// Print some helpful info to stderr for the user to see.
			// Output from stderr should be ignored from the completion script.
			fmt.Fprintf(os.Stderr, "Completion ended with directive: %s\n", directive.string())
			os.Exit(int(directive))
		},
	}
}

// CompDebug prints the specified string to the same file as where the
// completion script prints its logs.
// Note that completion printouts should never be on stdout as they would
// be wrongly interpreted as actual completion choices by the completion script.
func CompDebug(msg string) {
	msg = fmt.Sprintf("[Debug] %s", msg)

	// Such logs are only printed when the user has set the environment
	// variable BASH_COMP_DEBUG_FILE to the path of some file to be used.
	if path := os.Getenv("BASH_COMP_DEBUG_FILE"); path != "" {
		f, err := os.OpenFile(path,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			f.WriteString(msg)
		}
	}

	if debug {
		// Must print to stderr for this not to be read by the completion script.
		fmt.Fprintf(os.Stderr, msg)
	}
}

// CompDebugln prints the specified string with a newline at the end
// to the same file as where the completion script prints its logs.
// Such logs are only printed when the user has set the environment
// variable BASH_COMP_DEBUG_FILE to the path of some file to be used.
func CompDebugln(msg string) {
	CompDebug(fmt.Sprintf("%s\n", msg))
}

// CompError prints the specified completion message to stderr.
func CompError(msg string) {
	msg = fmt.Sprintf("[Error] %s", msg)

	CompDebug(msg)

	// If not already printed by the call to CompDebug().
	if !debug {
		// Must print to stderr for this not to be read by the completion script.
		fmt.Fprintf(os.Stderr, msg)
	}
}

// CompErrorln prints the specified completion message to stderr with a newline at the end.
func CompErrorln(msg string) {
	CompError(fmt.Sprintf("%s\n", msg))
}
