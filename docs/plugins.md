# The Helm Plugins Guide

Helm 2.1.0 introduced the concept of a client-side Helm _plugin_. A plugin is a
tool that can be accessed through the `helm` CLI, but which is not part of the
built-in Helm codebase.

Existing plugins can be found on [related](related.md#helm-plugins) section or by searching [Github](https://github.com/search?q=topic%3Ahelm-plugin&type=Repositories).

This guide explains how to use and create plugins.

## An Overview

Helm plugins are add-on tools that integrate seamlessly with Helm. They provide
a way to extend the core feature set of Helm, but without requiring every new
feature to be written in Go and added to the core tool.

Helm plugins have the following features:

- They can be added and removed from a Helm installation without impacting the
  core Helm tool.
- They can be written in any programming language.
- They integrate with Helm, and will show up in `helm help` and other places.

Helm plugins live in `$(helm home)/plugins`.

The Helm plugin model is partially modeled on Git's plugin model. To that end,
you may sometimes hear `helm` referred to as the _porcelain_ layer, with
plugins being the _plumbing_. This is a shorthand way of suggesting that
Helm provides the user experience and top level processing logic, while the
plugins do the "detail work" of performing a desired action.

## Installing a Plugin

Plugins are installed using the `$ helm plugin install <path|url>` command. You can pass in a path to a plugin on your local file system or a url of a remote VCS repo. The `helm plugin install` command clones or copies the plugin at the path/url given into `$ (helm home)/plugins`

```console
$ helm plugin install https://github.com/technosophos/helm-template
```

If you have a plugin tar distribution, simply untar the plugin into the
`$(helm home)/plugins` directory.

You can also install tarball plugins directly from url by issuing `helm plugin install http://domain/path/to/plugin.tar.gz`

## Building Plugins

In many ways, a plugin is similar to a chart. Each plugin has a top-level
directory, and then a `plugin.yaml` file.

```
$(helm home)/plugins/
  |- keybase/
      |
      |- plugin.yaml
      |- keybase.sh

```

In the example above, the `keybase` plugin is contained inside of a directory
named `keybase`. It has two files: `plugin.yaml` (required) and an executable
script, `keybase.sh` (optional).

The core of a plugin is a simple YAML file named `plugin.yaml`.
Here is a plugin YAML for a plugin that adds support for Keybase operations:

```
name: "keybase"
version: "0.1.0"
usage: "Integrate Keybase.io tools with Helm"
description: |-
  This plugin provides Keybase services to Helm.
ignoreFlags: false
useTunnel: false
command: "$HELM_PLUGIN_DIR/keybase.sh"
```

The `name` is the name of the plugin. When Helm executes it plugin, this is the
name it will use (e.g. `helm NAME` will invoke this plugin).

_`name` should match the directory name._ In our example above, that means the
plugin with `name: keybase` should be contained in a directory named `keybase`.

Restrictions on `name`:

- `name` cannot duplicate one of the existing `helm` top-level commands.
- `name` must be restricted to the characters ASCII a-z, A-Z, 0-9, `_` and `-`.

`version` is the SemVer 2 version of the plugin.
`usage` and `description` are both used to generate the help text of a command.

The `ignoreFlags` switch tells Helm to _not_ pass flags to the plugin. So if a
plugin is called with `helm myplugin --foo` and `ignoreFlags: true`, then `--foo`
is silently discarded.

The `useTunnel` switch indicates that the plugin needs a tunnel to Tiller. This
should be set to `true` _anytime a plugin talks to Tiller_. It will cause Helm
to open a tunnel, and then set `$TILLER_HOST` to the right local address for that
tunnel. But don't worry: if Helm detects that a tunnel is not necessary because
Tiller is running locally, it will not create the tunnel.

Finally, and most importantly, `command` is the command that this plugin will
execute when it is called. Environment variables are interpolated before the plugin
is executed. The pattern above illustrates the preferred way to indicate where
the plugin program lives.

There are some strategies for working with plugin commands:

- If a plugin includes an executable, the executable for a `command:` should be
  packaged in the plugin directory.
- The `command:` line will have any environment variables expanded before
  execution. `$HELM_PLUGIN_DIR` will point to the plugin directory.
- The command itself is not executed in a shell. So you can't oneline a shell script.
- Helm injects lots of configuration into environment variables. Take a look at
  the environment to see what information is available.
- Helm makes no assumptions about the language of the plugin. You can write it
  in whatever you prefer.
- Commands are responsible for implementing specific help text for `-h` and `--help`.
  Helm will use `usage` and `description` for `helm help` and `helm help myplugin`,
  but will not handle `helm myplugin --help`.

## Downloader Plugins
By default, Helm is able to fetch Charts using HTTP/S. As of Helm 2.4.0, plugins
can have a special capability to download Charts from arbitrary sources.

Plugins shall declare this special capability in the `plugin.yaml` file (top level):

```
downloaders:
- command: "bin/mydownloader"
  protocols:
  - "myprotocol"
  - "myprotocols"
```

If such plugin is installed, Helm can interact with the repository using the specified
protocol scheme by invoking the `command`. The special repository shall be added
similarily to the regular ones: `helm repo add favorite myprotocol://example.com/`
The rules for the special repos are the same to the regular ones: Helm must be able
to download the `index.yaml` file in order to discover and cache the list of
available Charts.

The defined command will be invoked with the following scheme:
`command certFile keyFile caFile full-URL`. The SSL credentials are coming from the
repo definition, stored in `$HELM_HOME/repository/repositories.yaml`. Downloader
plugin is expected to dump the raw content to stdout and report errors on stderr.

## Environment Variables

When Helm executes a plugin, it passes the outer environment to the plugin, and
also injects some additional environment variables.

Variables like `KUBECONFIG` are set for the plugin if they are set in the
outer environment.

The following variables are guaranteed to be set:

- `HELM_PLUGIN`: The path to the plugins directory
- `HELM_PLUGIN_NAME`: The name of the plugin, as invoked by `helm`. So
  `helm myplug` will have the short name `myplug`.
- `HELM_PLUGIN_DIR`: The directory that contains the plugin.
- `HELM_BIN`: The path to the `helm` command (as executed by the user).
- `HELM_HOME`: The path to the Helm home.
- `HELM_PATH_*`: Paths to important Helm files and directories are stored in
  environment variables prefixed by `HELM_PATH`.
- `TILLER_HOST`: The `domain:port` to Tiller. If a tunnel is created, this
  will point to the local endpoint for the tunnel. Otherwise, it will point
  to `$HELM_HOST`, `--host`, or the default host (according to Helm's rules of
  precedence).

While `HELM_HOST` _may_ be set, there is no guarantee that it will point to the
correct Tiller instance. This is done to allow plugin developer to access
`HELM_HOST` in its raw state when the plugin itself needs to manually configure
a connection.

## A Note on `useTunnel`

If a plugin specifies `useTunnel: true`, Helm will do the following (in order):

1. Parse global flags and the environment
2. Create the tunnel
3. Set `TILLER_HOST`
4. Execute the plugin
5. Close the tunnel

The tunnel is removed as soon as the `command` returns. So, for example, a
command cannot background a process and assume that that process will be able
to use the tunnel.

## A Note on Flag Parsing

When executing a plugin, Helm will parse global flags for its own use. Some of
these flags are _not_ passed on to the plugin.

- `--debug`: If this is specified, `$HELM_DEBUG` is set to `1`
- `--home`: This is converted to `$HELM_HOME`
- `--host`: This is converted to `$HELM_HOST`
- `--kube-context`: This is simply dropped. If your plugin uses `useTunnel`, this
  is used to set up the tunnel for you.

Plugins _should_ display help text and then exit for `-h` and `--help`. In all
other cases, plugins may use flags as appropriate.
