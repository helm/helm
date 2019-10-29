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

package main // import "helm.sh/helm/v3/cmd/helm"

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/output"
)

const (
	bashCompletionFunc = `
__helm_override_flag_list=(--kubeconfig --kube-context --namespace -n)
__helm_override_flags()
{
    local ${__helm_override_flag_list[*]##*-} two_word_of of var
    for w in "${words[@]}"; do
        if [ -n "${two_word_of}" ]; then
            eval "${two_word_of##*-}=\"${two_word_of}=\${w}\""
            two_word_of=
            continue
        fi
        for of in "${__helm_override_flag_list[@]}"; do
            case "${w}" in
                ${of}=*)
                    eval "${of##*-}=\"${w}\""
                    ;;
                ${of})
                    two_word_of="${of}"
                    ;;
            esac
        done
    done
    for var in "${__helm_override_flag_list[@]##*-}"; do
        if eval "test -n \"\$${var}\""; then
            eval "echo \${${var}}"
        fi
    done
}

__helm_override_flags_to_kubectl_flags()
{
    # --kubeconfig, -n, --namespace stay the same for kubectl
    # --kube-context becomes --context for kubectl
    __helm_debug "${FUNCNAME[0]}: flags to convert: $1"
    echo "$1" | sed s/kube-context/context/
}

__helm_get_repos()
{
    eval $(__helm_binary_name) repo list 2>/dev/null | tail +2 | cut -f1
}

__helm_get_contexts()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local template out
    template="{{ range .contexts  }}{{ .name }} {{ end }}"
    if out=$(kubectl config -o template --template="${template}" view 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_get_namespaces()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local template out
    template="{{ range .items  }}{{ .metadata.name }} {{ end }}"

    flags=$(__helm_override_flags_to_kubectl_flags "$(__helm_override_flags)")
    __helm_debug "${FUNCNAME[0]}: override flags for kubectl are: $flags"

    # Must use eval in case the flags contain a variable such as $HOME
    if out=$(eval kubectl get ${flags} -o template --template=\"${template}\" namespace 2>/dev/null); then
        COMPREPLY+=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_output_options()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    COMPREPLY+=( $( compgen -W "%[1]s" -- "$cur" ) )
}

__helm_binary_name()
{
    local helm_binary
    helm_binary="${words[0]}"
    __helm_debug "${FUNCNAME[0]}: helm_binary is ${helm_binary}"
    echo ${helm_binary}
}

# This function prevents the zsh shell from adding a space after
# a completion by adding a second, fake completion
__helm_zsh_comp_nospace() {
    __helm_debug "${FUNCNAME[0]}: in is ${in[*]}"

    local out in=("$@")

    # The shell will normally add a space after these completions.
    # To avoid that we should use "compopt -o nospace".  However, it is not
    # available in zsh.
    # Instead, we trick the shell by pretending there is a second, longer match.
    # We only do this if there is a single choice left for completion
    # to reduce the times the user could be presented with the fake completion choice.

    out=($(echo ${in[*]} | tr " " "\n" | \grep "^${cur}"))
    __helm_debug "${FUNCNAME[0]}: out is ${out[*]}"

    [ ${#out[*]} -eq 1 ] && out+=("${out}.")

    __helm_debug "${FUNCNAME[0]}: out is now ${out[*]}"

    echo "${out[*]}"
}

# $1 = 1 if the completion should include local charts (which means file completion)
__helm_list_charts()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local repo url file out=() nospace=0 wantFiles=$1

    # Handle completions for repos
    for repo in $(__helm_get_repos); do
        if [[ "${cur}" =~ ^${repo}/.* ]]; then
            # We are doing completion from within a repo
            out=$(eval $(__helm_binary_name) search repo ${cur} 2>/dev/null | cut -f1 | \grep ^${cur})
            nospace=0
        elif [[ ${repo} =~ ^${cur}.* ]]; then
            # We are completing a repo name
            out+=(${repo}/)
            nospace=1
        fi
    done
    __helm_debug "${FUNCNAME[0]}: out after repos is ${out[*]}"

    # Handle completions for url prefixes
    for url in https:// http:// file://; do
        if [[ "${cur}" =~ ^${url}.* ]]; then
            # The user already put in the full url prefix.  Return it
            # back as a completion to avoid the shell doing path completion
            out="${cur}"
            nospace=1
        elif [[ ${url} =~ ^${cur}.* ]]; then
            # We are completing a url prefix
            out+=(${url})
            nospace=1
        fi
    done
    __helm_debug "${FUNCNAME[0]}: out after urls is ${out[*]}"

    # Handle completion for files.
    # We only do this if:
    #   1- There are other completions found (if there are no completions,
    #      the shell will do file completion itself)
    #   2- If there is some input from the user (or else we will end up
    #      lising the entire content of the current directory which will
    #      be too many choices for the user to find the real repos)
    if [ $wantFiles -eq 1 ] && [ -n "${out[*]}" ] && [ -n "${cur}" ]; then
        for file in $(\ls); do
            if [[ ${file} =~ ^${cur}.* ]]; then
                # We are completing a file prefix
                out+=(${file})
                nospace=1
            fi
        done
    fi
    __helm_debug "${FUNCNAME[0]}: out after files is ${out[*]}"

    # If the user didn't provide any input to completion,
    # we provide a hint that a path can also be used
    [ $wantFiles -eq 1 ] && [ -z "${cur}" ] && out+=(./ /)

    __helm_debug "${FUNCNAME[0]}: out after checking empty input is ${out[*]}"

    if [ $nospace -eq 1 ]; then
        if [[ -n "${ZSH_VERSION}" ]]; then
            # Don't let the shell add a space after the completion
            local tmpout=$(__helm_zsh_comp_nospace "${out[@]}")
            unset out
            out=$tmpout
        elif [[ $(type -t compopt) = "builtin" ]]; then
            compopt -o nospace
        fi
    fi

    __helm_debug "${FUNCNAME[0]}: final out is ${out[*]}"
    COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
}

__helm_list_releases()
{
	__helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
	local out filter
	# Use ^ to map from the start of the release name
	filter="^${words[c]}"
    # Use eval in case helm_binary_name or __helm_override_flags contains a variable (e.g., $HOME/bin/h3)
    if out=$(eval $(__helm_binary_name) list $(__helm_override_flags) -a -q -m 1000 -f ${filter} 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_list_repos()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local out
    # Use eval in case helm_binary_name contains a variable (e.g., $HOME/bin/h3)
    if out=$(__helm_get_repos); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_list_plugins()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local out
    # Use eval in case helm_binary_name contains a variable (e.g., $HOME/bin/h3)
    if out=$(eval $(__helm_binary_name) plugin list 2>/dev/null | tail +2 | cut -f1); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_list_charts_after_name() {
    __helm_debug "${FUNCNAME[0]}: last_command is $last_command"
    if [[ ${#nouns[@]} -eq 1 ]]; then
        __helm_list_charts 1
    fi
}

__helm_list_releases_then_charts() {
    __helm_debug "${FUNCNAME[0]}: last_command is $last_command"
    if [[ ${#nouns[@]} -eq 0 ]]; then
        __helm_list_releases
    elif [[ ${#nouns[@]} -eq 1 ]]; then
        __helm_list_charts 1
    fi
}

__helm_custom_func()
{
    __helm_debug "${FUNCNAME[0]}: last_command is $last_command"
    case ${last_command} in
        helm_pull)
            __helm_list_charts 0
            return
            ;;
        helm_show_*)
            __helm_list_charts 1
            return
            ;;
        helm_install | helm_template)
            __helm_list_charts_after_name
            return
            ;;
        helm_upgrade)
            __helm_list_releases_then_charts
            return
            ;;
        helm_uninstall | helm_history | helm_status | helm_test |\
        helm_rollback | helm_get_*)
            __helm_list_releases
            return
            ;;
        helm_repo_remove)
            __helm_list_repos
            return
            ;;
        helm_plugin_remove | helm_plugin_update)
            __helm_list_plugins
            return
            ;;
        *)
            ;;
    esac
}
`
)

var (
	// Mapping of global flags that can have dynamic completion and the
	// completion function to be used.
	bashCompletionFlags = map[string]string{
		"namespace":    "__helm_get_namespaces",
		"kube-context": "__helm_get_contexts",
	}
)

var globalUsage = `The Kubernetes package manager

Common actions for Helm:

- helm search:    search for charts
- helm pull:      download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment variables:

+------------------+-----------------------------------------------------------------------------+
| Name             | Description                                                                 |
+------------------+-----------------------------------------------------------------------------+
| $XDG_CACHE_HOME  | set an alternative location for storing cached files.                       |
| $XDG_CONFIG_HOME | set an alternative location for storing Helm configuration.                 |
| $XDG_DATA_HOME   | set an alternative location for storing Helm data.                          |
| $HELM_DRIVER     | set the backend storage driver. Values are: configmap, secret, memory       |
| $HELM_NO_PLUGINS | disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.                  |
| $KUBECONFIG      | set an alternative Kubernetes configuration file (default "~/.kube/config") |
+------------------+-----------------------------------------------------------------------------+

Helm stores configuration based on the XDG base directory specification, so

- cached files are stored in $XDG_CACHE_HOME/helm
- configuration is stored in $XDG_CONFIG_HOME/helm
- data is stored in $XDG_DATA_HOME/helm

By default, the default directories depend on the Operating System. The defaults are listed below:

+------------------+---------------------------+--------------------------------+-------------------------+
| Operating System | Cache Path                | Configuration Path             | Data Path               |
+------------------+---------------------------+--------------------------------+-------------------------+
| Linux            | $HOME/.cache/helm         | $HOME/.config/helm             | $HOME/.local/share/helm |
| macOS            | $HOME/Library/Caches/helm | $HOME/Library/Preferences/helm | $HOME/Library/helm      |
| Windows          | %TEMP%\helm               | %APPDATA%\helm                 | %APPDATA%\helm          |
+------------------+---------------------------+--------------------------------+-------------------------+
`

func newRootCmd(actionConfig *action.Configuration, out io.Writer, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                    "helm",
		Short:                  "The Helm package manager for Kubernetes.",
		Long:                   globalUsage,
		SilenceUsage:           true,
		Args:                   require.NoArgs,
		BashCompletionFunction: fmt.Sprintf(bashCompletionFunc, strings.Join(output.Formats(), " ")),
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)

	// We can safely ignore any errors that flags.Parse encounters since
	// those errors will be caught later during the call to cmd.Execution.
	// This call is required to gather configuration information prior to
	// execution.
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.Parse(args)

	// Add subcommands
	cmd.AddCommand(
		// chart commands
		newCreateCmd(out),
		newDependencyCmd(out),
		newPullCmd(out),
		newShowCmd(out),
		newLintCmd(out),
		newPackageCmd(out),
		newRepoCmd(out),
		newSearchCmd(out),
		newVerifyCmd(out),

		// release commands
		newGetCmd(actionConfig, out),
		newHistoryCmd(actionConfig, out),
		newInstallCmd(actionConfig, out),
		newListCmd(actionConfig, out),
		newReleaseTestCmd(actionConfig, out),
		newRollbackCmd(actionConfig, out),
		newStatusCmd(actionConfig, out),
		newTemplateCmd(actionConfig, out),
		newUninstallCmd(actionConfig, out),
		newUpgradeCmd(actionConfig, out),

		newCompletionCmd(out),
		newEnvCmd(out),
		newPluginCmd(out),
		newVersionCmd(out),

		// Hidden documentation generator command: 'helm docs'
		newDocsCmd(out),
	)

	// Add annotation to flags for which we can generate completion choices
	for name, completion := range bashCompletionFlags {
		if cmd.Flag(name) != nil {
			if cmd.Flag(name).Annotations == nil {
				cmd.Flag(name).Annotations = map[string][]string{}
			}
			cmd.Flag(name).Annotations[cobra.BashCompCustom] = append(
				cmd.Flag(name).Annotations[cobra.BashCompCustom],
				completion,
			)
		}
	}

	// Add *experimental* subcommands
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptWriter(out),
	)
	if err != nil {
		// TODO: dont panic here, refactor newRootCmd to return error
		panic(err)
	}
	actionConfig.RegistryClient = registryClient
	cmd.AddCommand(
		newRegistryCmd(actionConfig, out),
		newChartCmd(actionConfig, out),
	)

	// Find and add plugins
	loadPlugins(cmd, out)

	return cmd
}
