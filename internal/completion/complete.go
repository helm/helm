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
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

// Global map allowing to find completion functions for commands or flags.
var validArgsFunctions = map[interface{}]func(cmd *cobra.Command, args []string, toComplete string) ([]string, BashCompDirective){}

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

// GetBashCustomFunction returns the bash code to handle custom go completion
// This should eventually be provided by Cobra
func GetBashCustomFunction() string {
	return fmt.Sprintf(`
__helm_custom_func()
{
    __helm_debug "${FUNCNAME[0]}: c is $c, words[@] is ${words[@]}, #words[@] is ${#words[@]}"
    __helm_debug "${FUNCNAME[0]}: cur is ${cur}, cword is ${cword}, words is ${words}"

    local out requestComp lastParam lastChar
    requestComp="${words[0]} %[1]s ${words[@]:1}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    __helm_debug "${FUNCNAME[0]}: lastParam ${lastParam}, lastChar ${lastChar}"

    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go method.
        __helm_debug "${FUNCNAME[0]}: Adding extra empty parameter"
        requestComp="${requestComp} \"\""
    fi

    __helm_debug "${FUNCNAME[0]}: calling ${requestComp}"
    # Use eval to handle any environment variables and such
    out=$(eval ${requestComp} 2>/dev/null)

    # Extract the directive int at the very end of the output following a :
    directive=${out##*:}
    # Remove the directive
    out=${out%%:*}
    if [ "${directive}" = "${out}" ]; then
        # There is not directive specified
        directive=0
    fi
    __helm_debug "${FUNCNAME[0]}: the completion directive is: ${directive}"
    __helm_debug "${FUNCNAME[0]}: the completions are: ${out[*]}"

    if [ $((${directive} & %[2]d)) -ne 0 ]; then
        __helm_debug "${FUNCNAME[0]}: received error, completion failed"
    else
        if [ $((${directive} & %[3]d)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __helm_debug "${FUNCNAME[0]}: activating no space"
                compopt -o nospace
            fi
        fi
        if [ $((${directive} & %[4]d)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __helm_debug "${FUNCNAME[0]}: activating no file completion"
                compopt +o default
            fi
        fi

        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${out[*]}" -- "$cur")
    fi
}
`, CompRequestCmd, BashCompDirectiveError, BashCompDirectiveNoSpace, BashCompDirectiveNoFileComp)
}

// RegisterValidArgsFunc should be called to register a function to provide argument completion for a command
func RegisterValidArgsFunc(cmd *cobra.Command, f func(cmd *cobra.Command, args []string, toComplete string) ([]string, BashCompDirective)) {
	if _, exists := validArgsFunctions[cmd]; exists {
		log.Fatal(fmt.Sprintf("RegisterValidArgsFunc: command '%s' already registered", cmd.Name()))
	}
	validArgsFunctions[cmd] = f
}

// RegisterFlagCompletionFunc should be called to register a function to provide completion for a flag
func RegisterFlagCompletionFunc(flag *pflag.Flag, f func(cmd *cobra.Command, args []string, toComplete string) ([]string, BashCompDirective)) {
	if _, exists := validArgsFunctions[flag]; exists {
		log.Fatal(fmt.Sprintf("RegisterFlagCompletionFunc: flag '%s' already registered", flag.Name))
	}
	validArgsFunctions[flag] = f

	// Make sure the completion script call the __helm_custom_func for the registered flag.
	// This is essential to make the = form work. E.g., helm -n=<TAB> or helm status --output=<TAB>
	if flag.Annotations == nil {
		flag.Annotations = map[string][]string{}
	}
	flag.Annotations[cobra.BashCompCustom] = []string{"__helm_custom_func"}
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
func NewCompleteCmd(settings *cli.EnvSettings, out io.Writer) *cobra.Command {
	debug = settings.Debug
	return &cobra.Command{
		Use:                   fmt.Sprintf("%s [command-line]", CompRequestCmd),
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
		Args:                  require.MinimumNArgs(1),
		Short:                 "Request shell completion choices for the specified command-line",
		Long: fmt.Sprintf("%s is a special command that is used by the shell completion logic\n%s",
			CompRequestCmd, "to request completion choices for the specified command-line."),
		Run: func(cmd *cobra.Command, args []string) {
			CompDebugln(fmt.Sprintf("%s was called with args %v", cmd.Name(), args))

			// The last argument, which is not complete, should not be part of the list of arguments
			toComplete := args[len(args)-1]
			trimmedArgs := args[:len(args)-1]

			// Find the real command for which completion must be performed
			finalCmd, finalArgs, err := cmd.Root().Find(trimmedArgs)
			if err != nil {
				// Unable to find the real command. E.g., helm invalidCmd <TAB>
				CompDebugln(fmt.Sprintf("Unable to find a command for arguments: %v", trimmedArgs))
				return
			}

			CompDebugln(fmt.Sprintf("Found final command '%s', with finalArgs %v", finalCmd.Name(), finalArgs))

			var flag *pflag.Flag
			if !finalCmd.DisableFlagParsing {
				// We only do flag completion if we are allowed to parse flags
				// This is important for helm plugins which need to do their own flag completion.
				flag, finalArgs, toComplete, err = checkIfFlagCompletion(finalCmd, finalArgs, toComplete)
				if err != nil {
					// Error while attempting to parse flags
					CompErrorln(err.Error())
					return
				}
			}

			// Parse the flags and extract the arguments to prepare for calling the completion function
			if err = finalCmd.ParseFlags(finalArgs); err != nil {
				CompErrorln(fmt.Sprintf("Error while parsing flags from args %v: %s", finalArgs, err.Error()))
				return
			}

			// We only remove the flags from the arguments if DisableFlagParsing is not set.
			// This is important for helm plugins, which need to receive all flags.
			// The plugin completion code will do its own flag parsing.
			if !finalCmd.DisableFlagParsing {
				finalArgs = finalCmd.Flags().Args()
				CompDebugln(fmt.Sprintf("Args without flags are '%v' with length %d", finalArgs, len(finalArgs)))
			}

			// Find completion function for the flag or command
			var key interface{}
			var keyStr string
			if flag != nil {
				key = flag
				keyStr = flag.Name
			} else {
				key = finalCmd
				keyStr = finalCmd.Name()
			}
			completionFn, ok := validArgsFunctions[key]
			if !ok {
				CompErrorln(fmt.Sprintf("Dynamic completion not supported/needed for flag or command: %s", keyStr))
				return
			}

			CompDebugln(fmt.Sprintf("Calling completion method for subcommand '%s' with args '%v' and toComplete '%s'", finalCmd.Name(), finalArgs, toComplete))
			completions, directive := completionFn(finalCmd, finalArgs, toComplete)
			for _, comp := range completions {
				// Print each possible completion to stdout for the completion script to consume.
				fmt.Fprintln(out, comp)
			}

			if directive > BashCompDirectiveError+BashCompDirectiveNoSpace+BashCompDirectiveNoFileComp {
				directive = BashCompDirectiveDefault
			}

			// As the last printout, print the completion directive for the
			// completion script to parse.
			// The directive integer must be that last character following a single :
			// The completion script expects :directive
			fmt.Fprintf(out, ":%d\n", directive)

			// Print some helpful info to stderr for the user to understand.
			// Output from stderr should be ignored from the completion script.
			fmt.Fprintf(os.Stderr, "Completion ended with directive: %s\n", directive.string())
		},
	}
}

func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

func checkIfFlagCompletion(finalCmd *cobra.Command, args []string, lastArg string) (*pflag.Flag, []string, string, error) {
	var flagName string
	trimmedArgs := args
	flagWithEqual := false
	if isFlag(lastArg) {
		if index := strings.Index(lastArg, "="); index >= 0 {
			flagName = strings.TrimLeft(lastArg[:index], "-")
			lastArg = lastArg[index+1:]
			flagWithEqual = true
		} else {
			return nil, nil, "", errors.New("Unexpected completion request for flag")
		}
	}

	if len(flagName) == 0 {
		if len(args) > 0 {
			prevArg := args[len(args)-1]
			if isFlag(prevArg) {
				// If the flag contains an = it means it has already been fully processed
				if index := strings.Index(prevArg, "="); index < 0 {
					flagName = strings.TrimLeft(prevArg, "-")

					// Remove the uncompleted flag or else Cobra could complain about
					// an invalid value for that flag e.g., helm status --output j<TAB>
					trimmedArgs = args[:len(args)-1]
				}
			}
		}
	}

	if len(flagName) == 0 {
		// Not doing flag completion
		return nil, trimmedArgs, lastArg, nil
	}

	flag := findFlag(finalCmd, flagName)
	if flag == nil {
		// Flag not supported by this command, nothing to complete
		err := fmt.Errorf("Subcommand '%s' does not support flag '%s'", finalCmd.Name(), flagName)
		return nil, nil, "", err
	}

	if !flagWithEqual {
		if len(flag.NoOptDefVal) != 0 {
			// We had assumed dealing with a two-word flag but the flag is a boolean flag.
			// In that case, there is no value following it, so we are not really doing flag completion.
			// Reset everything to do argument completion.
			trimmedArgs = args
			flag = nil
		}
	}

	return flag, trimmedArgs, lastArg, nil
}

func findFlag(cmd *cobra.Command, name string) *pflag.Flag {
	flagSet := cmd.Flags()
	if len(name) == 1 {
		// First convert the short flag into a long flag
		// as the cmd.Flag() search only accepts long flags
		if short := flagSet.ShorthandLookup(name); short != nil {
			CompDebugln(fmt.Sprintf("checkIfFlagCompletion: found flag '%s' which we will change to '%s'", name, short.Name))
			name = short.Name
		} else {
			set := cmd.InheritedFlags()
			if short = set.ShorthandLookup(name); short != nil {
				CompDebugln(fmt.Sprintf("checkIfFlagCompletion: found inherited flag '%s' which we will change to '%s'", name, short.Name))
				name = short.Name
			} else {
				return nil
			}
		}
	}
	return cmd.Flag(name)
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
		fmt.Fprintln(os.Stderr, msg)
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
		fmt.Fprintln(os.Stderr, msg)
	}
}

// CompErrorln prints the specified completion message to stderr with a newline at the end.
func CompErrorln(msg string) {
	CompError(fmt.Sprintf("%s\n", msg))
}
