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

const (
	// CompRequestCmd is the name of the hidden command that is used to request
	// completion results from helm.  It is used by the shell completion script.
	CompRequestCmd = "__complete"
	// CompWithDescRequestCmd is the name of the hidden command that is used to request
	// completion results with their description.  It is used by the shell completion script.
	CompWithDescRequestCmd = "__completeD"
)

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

    local out requestComp lastParam lastChar comp directive
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

// GenFishCompletion returns the fish script to handle completion fully
// This should eventually be provided by Cobra
func GenFishCompletion(out io.Writer, completionNoDesc bool) error {
	compCmd := CompWithDescRequestCmd
	if completionNoDesc {
		compCmd = CompRequestCmd
	}
	fishScript := fmt.Sprintf(`# fish completion for helm            -*- shell-script -*-

function __helm_debug
    set file "$BASH_COMP_DEBUG_FILE"
    if test -n "$file"
        echo "$argv" >> $file
    end
end

function __helm_comp_perform
    __helm_debug "Starting __helm_comp_perform with: $argv"

    set args (string split -- " " "$argv")
    set lastArg "$args[-1]"

    __helm_debug "args: $args"
    __helm_debug "last arg: $lastArg"

    set emptyArg ""
    if test -z "$lastArg"
        __helm_debug "Setting emptyArg"
        set emptyArg \"\"
    end
    __helm_debug "emptyArg: $emptyArg"

    set requestComp "$args[1] %[1]s $args[2..-1] $emptyArg"
    __helm_debug "Calling: $requestComp"

    set results (eval $requestComp 2> /dev/null)
    set comps $results[1..-2]
    set directiveLine $results[-1]

    # For Fish, when completing a flag with an = (e.g., helm -n=<TAB>)
    # completions must be prefixed with the flag
    set flagPrefix (string match -r -- '-.*=' "$lastArg")

    __helm_debug "Comps are: $comps"
    __helm_debug "DirectiveLine is: $directiveLine"
    __helm_debug "flagPrefix is: $flagPrefix"

    for comp in $comps
        printf "%%s%%s\n" "$flagPrefix" "$comp"
    end

    printf "%%s\n" "$directiveLine"
end

# This function does three things:
# 1- Obtain the completions and store them in the global __helm_comp_results
# 2- Set the __helm_comp_do_file_comp flag if file completion should be performed
#    and unset it otherwise
# 3- Return true if the completion results are not empty
function __helm_comp_prepare
    # Start fresh
    set --erase __helm_comp_do_file_comp
    set --erase __helm_comp_results

    # Check if the command-line is already provided.  This is useful for testing.
    if not set --query __helm_comp_commandLine
        set __helm_comp_commandLine (commandline)
    end
    __helm_debug "commandLine is: $__helm_comp_commandLine"

    set results (__helm_comp_perform "$__helm_comp_commandLine")
    set --erase __helm_comp_commandLine
    __helm_debug "Completion results: $results"

    if test -z "$results"
        __helm_debug "No completion, probably due to a failure"
        # Might as well do file completion, in case it helps
        set --global __helm_comp_do_file_comp 1
        return 0
    end

    set directive (string sub --start 2 $results[-1])
    set --global __helm_comp_results $results[1..-2]

    __helm_debug "Completions are: $__helm_comp_results"
    __helm_debug "Directive is: $directive"

    if test -z "$directive"
        set directive 0
    end

    set compErr (math (math --scale 0 $directive / %[2]d) %% 2)
    if test $compErr -eq 1
        __helm_debug "Received error directive: aborting."
        # Might as well do file completion, in case it helps
        set --global __helm_comp_do_file_comp 1
        return 0
    end

    set nospace (math (math --scale 0 $directive / %[3]d) %% 2)
    set nofiles (math (math --scale 0 $directive / %[4]d) %% 2)

    __helm_debug "nospace: $nospace, nofiles: $nofiles"

    # Important not to quote the variable for count to work
    set numComps (count $__helm_comp_results)
    __helm_debug "numComps: $numComps"

    if test $numComps -eq 1; and test $nospace -ne 0
        # To support the "nospace" directive we trick the shell
        # by outputting an extra, longer completion.
        __helm_debug "Adding second completion to perform nospace directive"
        set --append __helm_comp_results $__helm_comp_results[1].
    end

    if test $numComps -eq 0; and test $nofiles -eq 0
        __helm_debug "Requesting file completion"
        set --global __helm_comp_do_file_comp 1
    end

    # If we don't want file completion, we must return true even if there
    # are no completions found.  This is because fish will perform the last
    # completion command, even if its condition is false, if no other
    # completion command was triggered
    return (not set --query __helm_comp_do_file_comp)
end

# Remove any pre-existing helm completions since we will be handling all of them
complete -c helm -e

# The order in which the below two lines are defined is very important so that __helm_comp_prepare
# is called first.  It is __helm_comp_prepare that sets up the __helm_comp_do_file_comp variable.
#
# This completion will be run second as complete commands are added FILO.
# It triggers file completion choices when __helm_comp_do_file_comp is set.
complete -c helm -n 'set --query __helm_comp_do_file_comp'

# This completion will be run first as complete commands are added FILO.
# The call to __helm_comp_prepare will setup both __helm_comp_results abd __helm_comp_do_file_comp.
# It provides the program's completion choices.
complete -c helm -n '__helm_comp_prepare' -f -a '$__helm_comp_results'
`, compCmd, BashCompDirectiveError, BashCompDirectiveNoSpace, BashCompDirectiveNoFileComp)

	out.Write([]byte(fishScript))
	return nil
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

// NewCompleteCmd adds a special hidden command that an be used to request completions
func NewCompleteCmd(settings *cli.EnvSettings, out io.Writer) *cobra.Command {
	debug = settings.Debug
	return &cobra.Command{
		Use:                   fmt.Sprintf("%s [command-line]", CompRequestCmd),
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
		Aliases:               []string{CompWithDescRequestCmd},
		Args:                  require.MinimumNArgs(1),
		Short:                 "Request shell completion choices for the specified command-line",
		Long: fmt.Sprintf("%[2]s is a special command that is used by the shell completion logic\n%[1]s",
			"to request completion choices for the specified command-line.", CompRequestCmd),
		Run: func(cmd *cobra.Command, args []string) {
			completions, directive, err := getCompletions(cmd, args)
			if err != nil {
				CompErrorln(err.Error())
				// Keep going for multiple reasons:
				// 1- There could be some valid completions even though there was an error
				// 2- Even without completions, we need to print the directive
			}

			for _, comp := range completions {
				// Print each possible completion to stdout for the completion script to consume.
				fmt.Fprintln(out, comp)
			}

			if directive > BashCompDirectiveError+BashCompDirectiveNoSpace+BashCompDirectiveNoFileComp {
				directive = BashCompDirectiveDefault
			}

			// As the last printout, print the completion directive for the completion script to parse.
			// The directive integer must be that last character following a single colon (:).
			// The completion script expects :<directive>
			fmt.Fprintf(out, ":%d\n", directive)

			// Print some helpful info to stderr for the user to understand.
			// Output from stderr should be ignored by the completion script.
			fmt.Fprintf(os.Stderr, "Completion ended with directive: %s\n", directive.string())
		},
	}
}

func getCompletions(cmd *cobra.Command, args []string) ([]string, BashCompDirective, error) {
	var completions []string

	// The last argument, which is not completely typed by the user,
	// should not be part of the list of arguments
	toComplete := args[len(args)-1]
	trimmedArgs := args[:len(args)-1]

	// Find the real command for which completion must be performed
	finalCmd, finalArgs, err := cmd.Root().Find(trimmedArgs)
	if err != nil {
		// Unable to find the real command. E.g., helm invalidCmd <TAB>
		return completions, BashCompDirectiveDefault, fmt.Errorf("Unable to find a command for arguments: %v", trimmedArgs)
	}

	includeDesc := (cmd.CalledAs() == CompWithDescRequestCmd)
	if isFlag(toComplete) && !strings.Contains(toComplete, "=") {
		// We are completing a flag name
		finalCmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
			completions = append(completions, getFlagNameCompletions(flag, toComplete, includeDesc)...)
		})
		finalCmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
			completions = append(completions, getFlagNameCompletions(flag, toComplete, includeDesc)...)
		})

		directive := BashCompDirectiveDefault
		if len(completions) > 0 {
			if strings.HasSuffix(completions[0], "=") {
				directive = BashCompDirectiveNoSpace
			}
		}
		return completions, directive, nil
	}

	var flag *pflag.Flag
	if !finalCmd.DisableFlagParsing {
		// We only do flag completion if we are allowed to parse flags
		// This is important for helm plugins which need to do their own flag completion.
		flag, finalArgs, toComplete, err = checkIfFlagCompletion(finalCmd, finalArgs, toComplete)
		if err != nil {
			// Error while attempting to parse flags
			return completions, BashCompDirectiveDefault, err
		}
	}

	if flag == nil {
		subCmdExists := false
		// Complete subcommand names
		for _, subCmd := range finalCmd.Commands() {
			if subCmd.IsAvailableCommand() {
				subCmdExists = true
				if strings.HasPrefix(subCmd.Name(), toComplete) {
					comp := subCmd.Name()
					if includeDesc {
						comp = fmt.Sprintf("%s\t%s", comp, subCmd.Short)
					}
					completions = append(completions, comp)
				}
			}
		}
		if subCmdExists {
			// If there are sub-commands (even if they don't match), we stop completion; we shouldn't
			// do any argument completion.
			// A specific case where this is important is for plugin completion.
			// If we allowed to continue on, plugin dynamic completion would always be triggered
			// and would need to be handled by the plugin author.
			return completions, BashCompDirectiveNoFileComp, nil
		}

		// There are no sub-cmds, check for ValidArgs
		for _, validArg := range finalCmd.ValidArgs {
			if strings.HasPrefix(validArg, toComplete) {
				completions = append(completions, validArg)
			}
		}

		// Let the logic continue to add any ValidArgsFunction completions.
		// This is for commands that may choose to specify both ValidArgs and ValidArgsFunction.
	}

	// Parse the flags and extract the arguments to prepare for calling the completion function
	if err = finalCmd.ParseFlags(finalArgs); err != nil {
		return completions, BashCompDirectiveDefault, fmt.Errorf("Error while parsing flags from args %v: %s", finalArgs, err.Error())
	}

	// We only remove the flags from the arguments if DisableFlagParsing is not set.
	// This is important for helm plugins, which need to receive all flags.
	// The plugin completion code will do its own flag parsing.
	if !finalCmd.DisableFlagParsing {
		finalArgs = finalCmd.Flags().Args()
	}

	// Find completion function for the flag or command
	var key interface{}
	var nameStr string
	if flag != nil {
		key = flag
		nameStr = flag.Name
	} else {
		key = finalCmd
		nameStr = finalCmd.CommandPath()
	}
	completionFn, ok := validArgsFunctions[key]
	if !ok {
		CompDebugln(fmt.Sprintf("No dynamic completion registered for flag or command: %s", nameStr))
		// No custom completion registered.  This is ok.
		return completions, BashCompDirectiveDefault, nil
	}

	comps, directive := completionFn(finalCmd, finalArgs, toComplete)
	completions = append(completions, comps...)
	return completions, directive, nil
}

func getFlagNameCompletions(flag *pflag.Flag, toComplete string, includeDesc bool) []string {
	if nonCompletableFlag(flag) {
		return []string{}
	}

	var completions []string
	comp := "--" + flag.Name
	if strings.HasPrefix(comp, toComplete) {
		// Flag without the =
		completions = append(completions, comp)

		if len(flag.NoOptDefVal) == 0 {
			// Flag requires a value, so it can be suffixed with =
			comp += "="
			completions = append(completions, comp)
		}
	}

	comp = "-" + flag.Shorthand
	if len(flag.Shorthand) > 0 && strings.HasPrefix(comp, toComplete) {
		completions = append(completions, comp)
	}

	// Add documentation if requested
	if includeDesc {
		for idx, comp := range completions {
			completions[idx] = fmt.Sprintf("%s\t%s", comp, flag.Usage)
		}
	}
	return completions
}

func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}

func nonCompletableFlag(flag *pflag.Flag) bool {
	return flag.Hidden || flag.Deprecated != ""
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
				// Only consider the case where the flag does not contain an =.
				// If the flag contains an = it means it has already been fully processed,
				// so we don't need to deal with it here.
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
