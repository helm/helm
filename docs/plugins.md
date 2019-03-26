# The Helm Plugin Guide

This guide demonstrates how to install and write extensions for Helm. By thinking of core Helm commands as essential building blocks for interacting with a chart, a chart developer can think of plugins as a means of utilizing these building blocks to create more complex behavior. Plugins extend Helm with new sub-commands, allowing for new and custom features not included in Helm itself.

## An Overview

A plugin is nothing more than a standalone executable file, whose name begins with `helm-`. To install a plugin, simply move this executable file to anywhere on your PATH.

Helm plugins are add-on tools that integrate seamlessly with Helm. They provide a way to extend the core feature set of Helm, but without requiring every new feature to be written in Go and added to the core tool.

Helm plugins have the following features:

- They can be added and removed from a Helm installation without impacting the core Helm tool.
- They can be written in any programming language.

The Helm plugin model is partially modeled on Git's plugin model. To that end, you may sometimes hear `helm` referred to as the _porcelain_ layer, with plugins being the _plumbing_. This is a shorthand way of suggesting that Helm provides the user experience and top level processing logic, while the plugins do the "detail work" of performing a desired action.

## Installing a Plugin

Helm does not provide a plugin manager or anything similar to install or update plugins. It is your responsibility to ensure that plugin executables have a filename that begins with `helm-`, and that they are placed somewhere on your `$PATH`.

## Discovering Plugins

`helm plugin list` searches your PATH for plugins. Executing this command causes a traversal of all files in your PATH. Any files that are executable and begin with `helm-` will show up in the order in which they are present in your PATH in this command’s output. Any files beginning with `helm-` that are not executable will not be shown. Similar to how bash interprets duplicate commands in PATH, the first plugin that conflicts with another’s name will take precedence.

## Overriding Helm Commands

It is possible to create plugins that overwrite existing Helm commands. Creating a plugin called `helm-version` will take ownership of the command `helm version`, allowing you to extend Helm's capabilities or replace existing functionality with your own.

It is also possible to use plugins to add new subcommands to existing Helm commands. For example, adding a subcommand `helm create foo` by naming your plugin `helm-create-foo` will take ownership of the command `helm create foo`.

Do keep in mind that _plugins override all child subcommands as well, unless they were written as a plugin._ See more in the Limitations section listed below.

For example, if you write a plugin called `helm-dependency` to override `helm dependency`'s default behaviour, commands like `helm dependency build` are shadowed and unavailable to the user.

1. The `helm-dependency` plugin accepts the `build` argument
2. Another plugin called `help-dependency-build` is introduced.

## Limitations

Unless the plugin is overriding an existing command, Helm plugins can only be loaded one level deep from the root command tree.

This is because of how Helm loads plugins. Internally, Helm does a recursive search in its command subtree to determine where to inject the plugin into the CLI. If no existing command is found, Helm adds the plugin to the root of the command tree.

As an example, when Helm loads in a plugin called `helm-dependency-build`, it will find that `helm dependency build` already exists and will replace that command with the plugin.

if you write a plugin called `helm-dependency` to override `helm dependency`'s default behaviour, commands like `helm dependency build` are shadowed and unavailable to the user.

However, if *another* plugin implements `helm-dependency-build`, then `helm-dependency-build` will become available as `helm dependency build`, regardless if the parent command was overridden.

One last edge case with the plugin loader exists: unless another plugin implements the parent command, plugins two levels deep in the command tree will be loaded at the root level.

For example, if a plugin implements `helm-foo-bar` (where `helm-foo` is a Helm command that doesn't exist), then it will be loaded as `helm bar`. Again, this is because of how Helm loads plugins: If no existing command is found, Helm adds the plugin to the root of the command tree.

However, if another plugin implements `helm-foo`, then `helm-foo-bar` will be loaded as `helm foo bar`.

Because of this limitation, it is best to write plugins at the root level of the command subtree *unless* you are overriding the behaviour of a particular command, or you're introducing/replacing new commands to a particular plugin.

## Writing Plugins

You can write a plugin in any programming language or script that allows you to write command-line commands.

There is no plugin installation or pre-loading required. Plugin executables receive the inherited environment from Helm. A plugin determines which command path it wishes to implement based on its name. For example, a plugin wanting to provide a new command `helm foo`, would simply be named `helm-foo`, and live somewhere in the user’s PATH.

For example, you could write a bash script called `helm-foo`:

```
#!/bin/bash

# optional argument handling
if [[ "$1" == "version" ]]
then
    echo "1.0.0"
    exit 0
fi

# optional argument handling
if [[ "$1" == "config" ]]
then
    echo $KUBECONFIG
    exit 0
fi

echo "I am a plugin named helm-foo"
```

In the example above, the `helm-foo` plugin will accept `helm foo`, `helm foo version` and `helm foo config`.

## Downloader Plugins

By default, Helm is able to pull Charts using HTTP/S. However, plugins can extend Helm's capability to download Charts from arbitrary sources by registering as a downloader plugin.

Plugins can register themselves as a downloader plugin if the name begins with `helm-downloader-`.

If such plugin is installed, Helm can interact with the repository using the specified protocol scheme by invoking the plugin. The special repository shall be added similarly to the regular ones: `helm repo add favorite myprotocol://example.com/` The rules for the special repos are the same to the regular ones: Helm must be able to download the `index.yaml` file in order to discover and cache the list of available Charts.

The defined command will be invoked with the following scheme: `helm-downloader-myprotocol certFile keyFile caFile full-URL`. The SSL credentials are coming from the repo definition, stored in `$HELM_HOME/repository/repositories.yaml`. The downloader plugin is expected to dump the raw content to stdout and report errors on stderr.

## Environment Variables

When Helm executes a plugin, it passes the outer environment to the plugin, and also injects some additional environment variables.

Variables like `KUBECONFIG` are set for the plugin if they are set in the outer environment.

The following variables are guaranteed to be set:

- `HELM_PLUGIN_NAME`: The name of the plugin, as invoked by `helm`. So `helm myplug` will have the short name `myplug`.
- `HELM_BIN`: The path to the `helm` command (as executed by the user).
- `HELM_HOME`: The path to the Helm home.
- `HELM_PATH_*`: Paths to important Helm files and directories are stored in environment variables prefixed by `HELM_PATH`.

## A Note on Flag Parsing

When executing a plugin, Helm will parse global flags for its own use. Some of these flags are _not_ passed on to the plugin.

- `--debug`: If this is specified, `$HELM_DEBUG` is set to `1`
- `--home`: This is converted to `$HELM_HOME`.

Plugins _should_ display help text and then exit for `-h` and `--help`. In all other cases, plugins may use flags as appropriate.

## Changes from Helm 2

In Helm 2, plugins were installed using the `$ helm plugin install <path|url>` command. You could pass in a path to a plugin on your local file system or a url of a remote VCS repo. `helm plugin install` would clone or copy the plugin into `$(helm home)/plugins`.

Plugins also included a a `plugin.yaml` file which would define how to install, upgrade and invoke the plugin, along with hooks for the `helm` CLI to integrate the plugin into `helm help`.

This approach came with a few limitations, however:

- `helm plugin install --version 1.0.0` may fetch a plugin.yaml that did not match with the intended version number, leaving Helm in an erroneous state
- most VCS repositories only contained the source code, but not the binary. Most plugins just invoked a bash/powershell script to fetch the actual binary.

We were, in effect, re-building all of the functionality of a traditional package manager.

In the end, we decided to split out Helm's plugin manager from Helm 3, relying on the community to distribute their plugins through more traditional package managers.

This has the trade-off of plugins being unable to integrate with `helm help`, but the ease of development and ease of integrations with tools like `apt`/`brew` for package management and `man` for documentation made for a simpler plugin development experience.
