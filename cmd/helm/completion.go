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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/internal/completion"
)

const completionDesc = `
Generate autocompletions script for Helm for the specified shell (bash, zsh or fish).

This command can generate shell autocompletions. e.g.

    $ helm completion bash

Can be sourced as such

    $ source <(helm completion bash)
`

var (
	completionShells = map[string]func(out io.Writer, cmd *cobra.Command) error{
		"bash": runCompletionBash,
		"zsh":  runCompletionZsh,
		"fish": runCompletionFish,
	}
	completionNoDesc bool
)

func newCompletionCmd(out io.Writer) *cobra.Command {
	shells := []string{}
	for s := range completionShells {
		shells = append(shells, s)
	}

	cmd := &cobra.Command{
		Use:   "completion SHELL",
		Short: "Generate autocompletions script for the specified shell (bash, zsh or fish)",
		Long:  completionDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletion(out, cmd, args)
		},
		ValidArgs: shells,
	}
	cmd.PersistentFlags().BoolVar(&completionNoDesc, "no-descriptions", false, "disable completion description for shells that support it")

	return cmd
}

func runCompletion(out io.Writer, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("shell not specified")
	}
	if len(args) > 1 {
		return errors.New("too many arguments, expected only the shell type")
	}
	run, found := completionShells[args[0]]
	if !found {
		return errors.Errorf("unsupported shell type %q", args[0])
	}

	return run(out, cmd)
}

func runCompletionBash(out io.Writer, cmd *cobra.Command) error {
	err := cmd.Root().GenBashCompletion(out)

	// In case the user renamed the helm binary (e.g., to be able to run
	// both helm2 and helm3), we hook the new binary name to the completion function
	if binary := filepath.Base(os.Args[0]); binary != "helm" {
		renamedBinaryHook := `
# Hook the command used to generate the completion script
# to the helm completion function to handle the case where
# the user renamed the helm binary
if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_helm %[1]s
else
    complete -o default -o nospace -F __start_helm %[1]s
fi
`
		fmt.Fprintf(out, renamedBinaryHook, binary)
	}

	return err
}

func runCompletionZsh(out io.Writer, cmd *cobra.Command) error {
	zshInitialization := `#compdef helm

__helm_bash_source() {
	alias shopt=':'
	alias _expand=_bash_expand
	alias _complete=_bash_comp
	emulate -L sh
	setopt kshglob noshglob braceexpand
	source "$@"
}
__helm_type() {
	# -t is not supported by zsh
	if [ "$1" == "-t" ]; then
		shift
		# fake Bash 4 to disable "complete -o nospace". Instead
		# "compopt +-o nospace" is used in the code to toggle trailing
		# spaces. We don't support that, but leave trailing spaces on
		# all the time
		if [ "$1" = "__helm_compopt" ]; then
			echo builtin
			return 0
		fi
	fi
	type "$@"
}
__helm_compgen() {
	local completions w
	completions=( $(compgen "$@") ) || return $?
	# filter by given word as prefix
	while [[ "$1" = -* && "$1" != -- ]]; do
		shift
		shift
	done
	if [[ "$1" == -- ]]; then
		shift
	fi
	for w in "${completions[@]}"; do
		if [[ "${w}" = "$1"* ]]; then
			# Use printf instead of echo beause it is possible that
			# the value to print is -n, which would be interpreted
			# as a flag to echo
			printf "%s\n" "${w}"
		fi
	done
}
__helm_compopt() {
	true # don't do anything. Not supported by bashcompinit in zsh
}
__helm_ltrim_colon_completions()
{
	if [[ "$1" == *:* && "$COMP_WORDBREAKS" == *:* ]]; then
		# Remove colon-word prefix from COMPREPLY items
		local colon_word=${1%${1##*:}}
		local i=${#COMPREPLY[*]}
		while [[ $((--i)) -ge 0 ]]; do
			COMPREPLY[$i]=${COMPREPLY[$i]#"$colon_word"}
		done
	fi
}
__helm_get_comp_words_by_ref() {
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[${COMP_CWORD}-1]}"
	words=("${COMP_WORDS[@]}")
	cword=("${COMP_CWORD[@]}")
}
__helm_filedir() {
	local RET OLD_IFS w qw
	__debug "_filedir $@ cur=$cur"
	if [[ "$1" = \~* ]]; then
		# somehow does not work. Maybe, zsh does not call this at all
		eval echo "$1"
		return 0
	fi
	OLD_IFS="$IFS"
	IFS=$'\n'
	if [ "$1" = "-d" ]; then
		shift
		RET=( $(compgen -d) )
	else
		RET=( $(compgen -f) )
	fi
	IFS="$OLD_IFS"
	IFS="," __debug "RET=${RET[@]} len=${#RET[@]}"
	for w in ${RET[@]}; do
		if [[ ! "${w}" = "${cur}"* ]]; then
			continue
		fi
		if eval "[[ \"\${w}\" = *.$1 || -d \"\${w}\" ]]"; then
			qw="$(__helm_quote "${w}")"
			if [ -d "${w}" ]; then
				COMPREPLY+=("${qw}/")
			else
				COMPREPLY+=("${qw}")
			fi
		fi
	done
}
__helm_quote() {
	if [[ $1 == \'* || $1 == \"* ]]; then
		# Leave out first character
		printf %q "${1:1}"
	else
		printf %q "$1"
	fi
}
autoload -U +X bashcompinit && bashcompinit
# use word boundary patterns for BSD or GNU sed
LWORD='[[:<:]]'
RWORD='[[:>:]]'
if sed --help 2>&1 | grep -q 'GNU\|BusyBox'; then
	LWORD='\<'
	RWORD='\>'
fi
__helm_convert_bash_to_zsh() {
	sed \
	-e 's/declare -F/whence -w/' \
	-e 's/_get_comp_words_by_ref "\$@"/_get_comp_words_by_ref "\$*"/' \
	-e 's/local \([a-zA-Z0-9_]*\)=/local \1; \1=/' \
	-e 's/flags+=("\(--.*\)=")/flags+=("\1"); two_word_flags+=("\1")/' \
	-e 's/must_have_one_flag+=("\(--.*\)=")/must_have_one_flag+=("\1")/' \
	-e "s/${LWORD}_filedir${RWORD}/__helm_filedir/g" \
	-e "s/${LWORD}_get_comp_words_by_ref${RWORD}/__helm_get_comp_words_by_ref/g" \
	-e "s/${LWORD}__ltrim_colon_completions${RWORD}/__helm_ltrim_colon_completions/g" \
	-e "s/${LWORD}compgen${RWORD}/__helm_compgen/g" \
	-e "s/${LWORD}compopt${RWORD}/__helm_compopt/g" \
	-e "s/${LWORD}declare${RWORD}/builtin declare/g" \
	-e "s/\\\$(type${RWORD}/\$(__helm_type/g" \
	-e 's/aliashash\["\(.\{1,\}\)"\]/aliashash[\1]/g' \
	-e 's/FUNCNAME/funcstack/g' \
	<<'BASH_COMPLETION_EOF'
`
	out.Write([]byte(zshInitialization))

	runCompletionBash(out, cmd)

	zshTail := `
BASH_COMPLETION_EOF
}
__helm_bash_source <(__helm_convert_bash_to_zsh)
`
	out.Write([]byte(zshTail))
	return nil
}

func runCompletionFish(out io.Writer, cmd *cobra.Command) error {
	compCmd := completion.CompWithDescRequestCmd
	if completionNoDesc {
		compCmd = completion.CompRequestCmd
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
`, compCmd, completion.BashCompDirectiveError, completion.BashCompDirectiveNoSpace, completion.BashCompDirectiveNoFileComp)

	completeCmds := `
# Remove any pre-existing helm completions since we will be handling all of them
complete -c %[1]s -e

# The order in which the below two lines are defined is very important so that __helm_comp_prepare
# is called first.  It is __helm_comp_prepare that sets up the __helm_comp_do_file_comp variable.
#
# This completion will be run second as complete commands are added FILO.
# It triggers file completion choices when __helm_comp_do_file_comp is set.
complete -c %[1]s -n 'set --query __helm_comp_do_file_comp'

# This completion will be run first as complete commands are added FILO.
# The call to __helm_comp_prepare will setup both __helm_comp_results abd __helm_comp_do_file_comp.
# It provides the program's completion choices.
complete -c %[1]s -n '__helm_comp_prepare' -f -a '$__helm_comp_results'
`
	out.Write([]byte(fishScript))
	out.Write([]byte(fmt.Sprintf(completeCmds, "helm")))

	// In case the user renamed the helm binary (e.g., to be able to run
	// both helm2 and helm3), we hook the new binary name to the completion function
	if binary := filepath.Base(os.Args[0]); binary != "helm" {
		out.Write([]byte(fmt.Sprintf(completeCmds, binary)))
	}
	return nil
}
