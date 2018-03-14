# bash completion for helm                                 -*- shell-script -*-

__debug()
{
    if [[ -n ${BASH_COMP_DEBUG_FILE} ]]; then
        echo "$*" >> "${BASH_COMP_DEBUG_FILE}"
    fi
}

# Homebrew on Macs have version 1.3 of bash-completion which doesn't include
# _init_completion. This is a very minimal version of that function.
__my_init_completion()
{
    COMPREPLY=()
    _get_comp_words_by_ref "$@" cur prev words cword
}

__index_of_word()
{
    local w word=$1
    shift
    index=0
    for w in "$@"; do
        [[ $w = "$word" ]] && return
        index=$((index+1))
    done
    index=-1
}

__contains_word()
{
    local w word=$1; shift
    for w in "$@"; do
        [[ $w = "$word" ]] && return
    done
    return 1
}

__handle_reply()
{
    __debug "${FUNCNAME[0]}"
    case $cur in
        -*)
            if [[ $(type -t compopt) = "builtin" ]]; then
                compopt -o nospace
            fi
            local allflags
            if [ ${#must_have_one_flag[@]} -ne 0 ]; then
                allflags=("${must_have_one_flag[@]}")
            else
                allflags=("${flags[*]} ${two_word_flags[*]}")
            fi
            COMPREPLY=( $(compgen -W "${allflags[*]}" -- "$cur") )
            if [[ $(type -t compopt) = "builtin" ]]; then
                [[ "${COMPREPLY[0]}" == *= ]] || compopt +o nospace
            fi

            # complete after --flag=abc
            if [[ $cur == *=* ]]; then
                if [[ $(type -t compopt) = "builtin" ]]; then
                    compopt +o nospace
                fi

                local index flag
                flag="${cur%%=*}"
                __index_of_word "${flag}" "${flags_with_completion[@]}"
                if [[ ${index} -ge 0 ]]; then
                    COMPREPLY=()
                    PREFIX=""
                    cur="${cur#*=}"
                    ${flags_completion[${index}]}
                    if [ -n "${ZSH_VERSION}" ]; then
                        # zfs completion needs --flag= prefix
                        eval "COMPREPLY=( \"\${COMPREPLY[@]/#/${flag}=}\" )"
                    fi
                fi
            fi
            return 0;
            ;;
    esac

    # check if we are handling a flag with special work handling
    local index
    __index_of_word "${prev}" "${flags_with_completion[@]}"
    if [[ ${index} -ge 0 ]]; then
        ${flags_completion[${index}]}
        return
    fi

    # we are parsing a flag and don't have a special handler, no completion
    if [[ ${cur} != "${words[cword]}" ]]; then
        return
    fi

    local completions
    completions=("${commands[@]}")
    if [[ ${#must_have_one_noun[@]} -ne 0 ]]; then
        completions=("${must_have_one_noun[@]}")
    fi
    if [[ ${#must_have_one_flag[@]} -ne 0 ]]; then
        completions+=("${must_have_one_flag[@]}")
    fi
    COMPREPLY=( $(compgen -W "${completions[*]}" -- "$cur") )

    if [[ ${#COMPREPLY[@]} -eq 0 && ${#noun_aliases[@]} -gt 0 && ${#must_have_one_noun[@]} -ne 0 ]]; then
        COMPREPLY=( $(compgen -W "${noun_aliases[*]}" -- "$cur") )
    fi

    if [[ ${#COMPREPLY[@]} -eq 0 ]]; then
        declare -F __custom_func >/dev/null && __custom_func
    fi

    __ltrim_colon_completions "$cur"
}

# The arguments should be in the form "ext1|ext2|extn"
__handle_filename_extension_flag()
{
    local ext="$1"
    _filedir "@(${ext})"
}

__handle_subdirs_in_dir_flag()
{
    local dir="$1"
    pushd "${dir}" >/dev/null 2>&1 && _filedir -d && popd >/dev/null 2>&1
}

__handle_flag()
{
    __debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    # if a command required a flag, and we found it, unset must_have_one_flag()
    local flagname=${words[c]}
    local flagvalue
    # if the word contained an =
    if [[ ${words[c]} == *"="* ]]; then
        flagvalue=${flagname#*=} # take in as flagvalue after the =
        flagname=${flagname%%=*} # strip everything after the =
        flagname="${flagname}=" # but put the = back
    fi
    __debug "${FUNCNAME[0]}: looking for ${flagname}"
    if __contains_word "${flagname}" "${must_have_one_flag[@]}"; then
        must_have_one_flag=()
    fi

    # if you set a flag which only applies to this command, don't show subcommands
    if __contains_word "${flagname}" "${local_nonpersistent_flags[@]}"; then
      commands=()
    fi

    # keep flag value with flagname as flaghash
    if [ -n "${flagvalue}" ] ; then
        flaghash[${flagname}]=${flagvalue}
    elif [ -n "${words[ $((c+1)) ]}" ] ; then
        flaghash[${flagname}]=${words[ $((c+1)) ]}
    else
        flaghash[${flagname}]="true" # pad "true" for bool flag
    fi

    # skip the argument to a two word flag
    if __contains_word "${words[c]}" "${two_word_flags[@]}"; then
        c=$((c+1))
        # if we are looking for a flags value, don't show commands
        if [[ $c -eq $cword ]]; then
            commands=()
        fi
    fi

    c=$((c+1))

}

__handle_noun()
{
    __debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    if __contains_word "${words[c]}" "${must_have_one_noun[@]}"; then
        must_have_one_noun=()
    elif __contains_word "${words[c]}" "${noun_aliases[@]}"; then
        must_have_one_noun=()
    fi

    nouns+=("${words[c]}")
    c=$((c+1))
}

__handle_command()
{
    __debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    local next_command
    if [[ -n ${last_command} ]]; then
        next_command="_${last_command}_${words[c]//:/__}"
    else
        if [[ $c -eq 0 ]]; then
            next_command="_$(basename "${words[c]//:/__}")"
        else
            next_command="_${words[c]//:/__}"
        fi
    fi
    c=$((c+1))
    __debug "${FUNCNAME[0]}: looking for ${next_command}"
    declare -F $next_command >/dev/null && $next_command
}

__handle_word()
{
    if [[ $c -ge $cword ]]; then
        __handle_reply
        return
    fi
    __debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __handle_flag
    elif __contains_word "${words[c]}" "${commands[@]}"; then
        __handle_command
    elif [[ $c -eq 0 ]] && __contains_word "$(basename "${words[c]}")" "${commands[@]}"; then
        __handle_command
    else
        __handle_noun
    fi
    __handle_word
}

_helm_completion()
{
    last_command="helm_completion"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    must_have_one_noun+=("bash")
    must_have_one_noun+=("zsh")
    noun_aliases=()
}

_helm_create()
{
    last_command="helm_create"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--starter=")
    two_word_flags+=("-p")
    local_nonpersistent_flags+=("--starter=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_delete()
{
    last_command="helm_delete"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--no-hooks")
    local_nonpersistent_flags+=("--no-hooks")
    flags+=("--purge")
    local_nonpersistent_flags+=("--purge")
    flags+=("--timeout=")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_dependency_build()
{
    last_command="helm_dependency_build"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_dependency_list()
{
    last_command="helm_dependency_list"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_dependency_update()
{
    last_command="helm_dependency_update"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--skip-refresh")
    local_nonpersistent_flags+=("--skip-refresh")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_dependency()
{
    last_command="helm_dependency"
    commands=()
    commands+=("build")
    commands+=("list")
    commands+=("update")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_fetch()
{
    last_command="helm_fetch"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--destination=")
    two_word_flags+=("-d")
    local_nonpersistent_flags+=("--destination=")
    flags+=("--devel")
    local_nonpersistent_flags+=("--devel")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--prov")
    local_nonpersistent_flags+=("--prov")
    flags+=("--repo=")
    local_nonpersistent_flags+=("--repo=")
    flags+=("--untar")
    local_nonpersistent_flags+=("--untar")
    flags+=("--untardir=")
    local_nonpersistent_flags+=("--untardir=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_get_hooks()
{
    last_command="helm_get_hooks"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--revision=")
    local_nonpersistent_flags+=("--revision=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_get_manifest()
{
    last_command="helm_get_manifest"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--revision=")
    local_nonpersistent_flags+=("--revision=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_get_values()
{
    last_command="helm_get_values"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--all")
    flags+=("-a")
    local_nonpersistent_flags+=("--all")
    flags+=("--revision=")
    local_nonpersistent_flags+=("--revision=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_get()
{
    last_command="helm_get"
    commands=()
    commands+=("hooks")
    commands+=("manifest")
    commands+=("values")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--revision=")
    local_nonpersistent_flags+=("--revision=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_history()
{
    last_command="helm_history"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--max=")
    local_nonpersistent_flags+=("--max=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_home()
{
    last_command="helm_home"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_init()
{
    last_command="helm_init"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--canary-image")
    local_nonpersistent_flags+=("--canary-image")
    flags+=("--client-only")
    flags+=("-c")
    local_nonpersistent_flags+=("--client-only")
    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--local-repo-url=")
    local_nonpersistent_flags+=("--local-repo-url=")
    flags+=("--net-host")
    local_nonpersistent_flags+=("--net-host")
    flags+=("--service-account=")
    local_nonpersistent_flags+=("--service-account=")
    flags+=("--skip-refresh")
    local_nonpersistent_flags+=("--skip-refresh")
    flags+=("--stable-repo-url=")
    local_nonpersistent_flags+=("--stable-repo-url=")
    flags+=("--tiller-image=")
    two_word_flags+=("-i")
    local_nonpersistent_flags+=("--tiller-image=")
    flags+=("--tiller-tls")
    local_nonpersistent_flags+=("--tiller-tls")
    flags+=("--tiller-tls-cert=")
    local_nonpersistent_flags+=("--tiller-tls-cert=")
    flags+=("--tiller-tls-key=")
    local_nonpersistent_flags+=("--tiller-tls-key=")
    flags+=("--tiller-tls-verify")
    local_nonpersistent_flags+=("--tiller-tls-verify")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--upgrade")
    local_nonpersistent_flags+=("--upgrade")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_inspect_chart()
{
    last_command="helm_inspect_chart"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--repo=")
    local_nonpersistent_flags+=("--repo=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_inspect_values()
{
    last_command="helm_inspect_values"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--repo=")
    local_nonpersistent_flags+=("--repo=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_inspect()
{
    last_command="helm_inspect"
    commands=()
    commands+=("chart")
    commands+=("values")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--repo=")
    local_nonpersistent_flags+=("--repo=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_install()
{
    last_command="helm_install"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--devel")
    local_nonpersistent_flags+=("--devel")
    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--name=")
    two_word_flags+=("-n")
    local_nonpersistent_flags+=("--name=")
    flags+=("--name-template=")
    local_nonpersistent_flags+=("--name-template=")
    flags+=("--namespace=")
    local_nonpersistent_flags+=("--namespace=")
    flags+=("--no-hooks")
    local_nonpersistent_flags+=("--no-hooks")
    flags+=("--replace")
    local_nonpersistent_flags+=("--replace")
    flags+=("--repo=")
    local_nonpersistent_flags+=("--repo=")
    flags+=("--set=")
    local_nonpersistent_flags+=("--set=")
    flags+=("--set-string=")
    local_nonpersistent_flags+=("--set-string=")
    flags+=("--timeout=")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--values=")
    two_word_flags+=("-f")
    local_nonpersistent_flags+=("--values=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--wait")
    local_nonpersistent_flags+=("--wait")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_lint()
{
    last_command="helm_lint"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--strict")
    local_nonpersistent_flags+=("--strict")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_list()
{
    last_command="helm_list"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--all")
    local_nonpersistent_flags+=("--all")
    flags+=("--date")
    flags+=("-d")
    local_nonpersistent_flags+=("--date")
    flags+=("--deleted")
    local_nonpersistent_flags+=("--deleted")
    flags+=("--deleting")
    local_nonpersistent_flags+=("--deleting")
    flags+=("--deployed")
    local_nonpersistent_flags+=("--deployed")
    flags+=("--failed")
    local_nonpersistent_flags+=("--failed")
    flags+=("--max=")
    two_word_flags+=("-m")
    local_nonpersistent_flags+=("--max=")
    flags+=("--namespace=")
    local_nonpersistent_flags+=("--namespace=")
    flags+=("--offset=")
    two_word_flags+=("-o")
    local_nonpersistent_flags+=("--offset=")
    flags+=("--reverse")
    flags+=("-r")
    local_nonpersistent_flags+=("--reverse")
    flags+=("--short")
    flags+=("-q")
    local_nonpersistent_flags+=("--short")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_package()
{
    last_command="helm_package"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--destination=")
    two_word_flags+=("-d")
    local_nonpersistent_flags+=("--destination=")
    flags+=("--key=")
    local_nonpersistent_flags+=("--key=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--save")
    local_nonpersistent_flags+=("--save")
    flags+=("--sign")
    local_nonpersistent_flags+=("--sign")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_plugin_install()
{
    last_command="helm_plugin_install"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_plugin_list()
{
    last_command="helm_plugin_list"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_plugin_remove()
{
    last_command="helm_plugin_remove"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_plugin_update()
{
    last_command="helm_plugin_update"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_plugin()
{
    last_command="helm_plugin"
    commands=()
    commands+=("install")
    commands+=("list")
    commands+=("remove")
    commands+=("update")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_repo_add()
{
    last_command="helm_repo_add"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--no-update")
    local_nonpersistent_flags+=("--no-update")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_repo_index()
{
    last_command="helm_repo_index"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--merge=")
    local_nonpersistent_flags+=("--merge=")
    flags+=("--url=")
    local_nonpersistent_flags+=("--url=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_repo_list()
{
    last_command="helm_repo_list"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_repo_remove()
{
    last_command="helm_repo_remove"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_repo_update()
{
    last_command="helm_repo_update"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_repo()
{
    last_command="helm_repo"
    commands=()
    commands+=("add")
    commands+=("index")
    commands+=("list")
    commands+=("remove")
    commands+=("update")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_reset()
{
    last_command="helm_reset"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--force")
    flags+=("-f")
    local_nonpersistent_flags+=("--force")
    flags+=("--remove-helm-home")
    local_nonpersistent_flags+=("--remove-helm-home")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_rollback()
{
    last_command="helm_rollback"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--force")
    local_nonpersistent_flags+=("--force")
    flags+=("--no-hooks")
    local_nonpersistent_flags+=("--no-hooks")
    flags+=("--recreate-pods")
    local_nonpersistent_flags+=("--recreate-pods")
    flags+=("--timeout=")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--wait")
    local_nonpersistent_flags+=("--wait")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_search()
{
    last_command="helm_search"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--regexp")
    flags+=("-r")
    local_nonpersistent_flags+=("--regexp")
    flags+=("--version=")
    two_word_flags+=("-v")
    local_nonpersistent_flags+=("--version=")
    flags+=("--versions")
    flags+=("-l")
    local_nonpersistent_flags+=("--versions")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_serve()
{
    last_command="helm_serve"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address=")
    local_nonpersistent_flags+=("--address=")
    flags+=("--repo-path=")
    local_nonpersistent_flags+=("--repo-path=")
    flags+=("--url=")
    local_nonpersistent_flags+=("--url=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_status()
{
    last_command="helm_status"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--revision=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_test()
{
    last_command="helm_test"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--cleanup")
    local_nonpersistent_flags+=("--cleanup")
    flags+=("--timeout=")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_upgrade()
{
    last_command="helm_upgrade"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--ca-file=")
    local_nonpersistent_flags+=("--ca-file=")
    flags+=("--cert-file=")
    local_nonpersistent_flags+=("--cert-file=")
    flags+=("--devel")
    local_nonpersistent_flags+=("--devel")
    flags+=("--disable-hooks")
    local_nonpersistent_flags+=("--disable-hooks")
    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--force")
    local_nonpersistent_flags+=("--force")
    flags+=("--install")
    flags+=("-i")
    local_nonpersistent_flags+=("--install")
    flags+=("--key-file=")
    local_nonpersistent_flags+=("--key-file=")
    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--namespace=")
    local_nonpersistent_flags+=("--namespace=")
    flags+=("--no-hooks")
    local_nonpersistent_flags+=("--no-hooks")
    flags+=("--recreate-pods")
    local_nonpersistent_flags+=("--recreate-pods")
    flags+=("--repo=")
    local_nonpersistent_flags+=("--repo=")
    flags+=("--reset-values")
    local_nonpersistent_flags+=("--reset-values")
    flags+=("--reuse-values")
    local_nonpersistent_flags+=("--reuse-values")
    flags+=("--set=")
    local_nonpersistent_flags+=("--set=")
    flags+=("--timeout=")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--values=")
    two_word_flags+=("-f")
    local_nonpersistent_flags+=("--values=")
    flags+=("--verify")
    local_nonpersistent_flags+=("--verify")
    flags+=("--version=")
    local_nonpersistent_flags+=("--version=")
    flags+=("--wait")
    local_nonpersistent_flags+=("--wait")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_verify()
{
    last_command="helm_verify"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--keyring=")
    local_nonpersistent_flags+=("--keyring=")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm_version()
{
    last_command="helm_version"
    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--client")
    flags+=("-c")
    local_nonpersistent_flags+=("--client")
    flags+=("--server")
    flags+=("-s")
    local_nonpersistent_flags+=("--server")
    flags+=("--short")
    local_nonpersistent_flags+=("--short")
    flags+=("--tls")
    local_nonpersistent_flags+=("--tls")
    flags+=("--tls-ca-cert=")
    local_nonpersistent_flags+=("--tls-ca-cert=")
    flags+=("--tls-cert=")
    local_nonpersistent_flags+=("--tls-cert=")
    flags+=("--tls-key=")
    local_nonpersistent_flags+=("--tls-key=")
    flags+=("--tls-verify")
    local_nonpersistent_flags+=("--tls-verify")
    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_helm()
{
    last_command="helm"
    commands=()
    commands+=("completion")
    commands+=("create")
    commands+=("delete")
    commands+=("dependency")
    commands+=("fetch")
    commands+=("get")
    commands+=("history")
    commands+=("home")
    commands+=("init")
    commands+=("inspect")
    commands+=("install")
    commands+=("lint")
    commands+=("list")
    commands+=("package")
    commands+=("plugin")
    commands+=("repo")
    commands+=("reset")
    commands+=("rollback")
    commands+=("search")
    commands+=("serve")
    commands+=("status")
    commands+=("test")
    commands+=("upgrade")
    commands+=("verify")
    commands+=("version")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--debug")
    flags+=("--home=")
    flags+=("--host=")
    flags+=("--kube-context=")
    flags+=("--tiller-namespace=")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

__start_helm()
{
    local cur prev words cword
    declare -A flaghash 2>/dev/null || :
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion -s || return
    else
        __my_init_completion -n "=" || return
    fi

    local c=0
    local flags=()
    local two_word_flags=()
    local local_nonpersistent_flags=()
    local flags_with_completion=()
    local flags_completion=()
    local commands=("helm")
    local must_have_one_flag=()
    local must_have_one_noun=()
    local last_command
    local nouns=()

    __handle_word
}

if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_helm helm
else
    complete -o default -o nospace -F __start_helm helm
fi

# ex: ts=4 sw=4 et filetype=sh
