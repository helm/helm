# Built-in Objects

Objects are passed into a template from the template engine. And your code can pass objects around (we'll see examples when we look at the `with` and `range` statements). There are even a few ways to create new objects within your templates, like with the `tuple` function we'll see later.

Objects can be simple, and have just one value. Or they can contain other objects or functions. For example. the `Release` object contains several objects (like `Release.Name`) and the `Files` object has a few functions.

In the previous section, we use `{{.Release.Name}}` to insert the name of a release into a template. `Release` is one of the top-level objects that you can access in your templates.

- `Release`: This object describes the release itself. It has several objects inside of it:
  - `Release.Name`: The release name
  - `Release.Time`: The time of the release
  - `Release.Namespace`: The namespace to be released into (if the manifest doesn't override)
  - `Release.Service`: The name of the releasing service (always `Tiller`).
  - `Release.Revision`: The revision number of this release. It begins at 1 and is incremented for each `helm upgrade`.
  - `Release.IsUpgrade`: This is set to `true` if the current operation is an upgrade or rollback.
  - `Release.IsInstall`: This is set to `true` if the current operation is an install.
- `Values`: Values passed into the template from the `values.yaml` file and from user-supplied files. By default, `Values` is empty.
- `Chart`: The contents of the `Chart.yaml` file. Any data in `Chart.yaml` will be accessible here. For example `{{.Chart.Name}}-{{.Chart.Version}}` will print out the `mychart-0.1.0`.
  - The available fields are listed in the [Charts Guide](https://github.com/kubernetes/helm/blob/master/docs/charts.md#the-chartyaml-file)
- `Files`: This provides access to all non-special files in a chart. While you cannot use it to access templates, you can use it to access other files in the chart. See the section _Accessing Files_ for more.
  - `Files.Get` is a function for getting a file by name (`.Files.Get config.ini`)
  - `Files.GetBytes` is a function for getting the contents of a file as an array of bytes instead of as a string. This is useful for things like images.
- `Capabilities`: This provides information about what capabilities the Kubernetes cluster supports.
  - `Capabilities.APIVersions` is a set of versions.
  - `Capabilities.APIVersions.Has $version` indicates whether a version (`batch/v1`) is enabled on the cluster.
  - `Capabilities.KubeVersion` provides a way to look up the Kubernetes version. It has the following values: `Major`, `Minor`, `GitVersion`, `GitCommit`, `GitTreeState`, `BuildDate`, `GoVersion`, `Compiler`, and `Platform`.
  - `Capabilities.TillerVersion` provides a way to look up the Tiller version. It has the following values: `SemVer`, `GitCommit`, and `GitTreeState`.
- `Template`: Contains information about the current template that is being executed
  - `Name`: A namespaced filepath to the current template (e.g. `mychart/templates/mytemplate.yaml`)
  - `BasePath`: The namespaced path to the templates directory of the current chart (e.g. `mychart/templates`). This can be used to [include template files](https://github.com/kubernetes/helm/blob/master/docs/charts_tips_and_tricks.md#automatically-roll-deployments-when-configmaps-or-secrets-change)

The values are available to any top-level template. As we will see later, this does not necessarily mean that they will be available _everywhere_.

The built-in values always begin with a capital letter. This is in keeping with Go's naming convention. When you create your own names, you are free to use a convention that suits your team. Some teams, like the [Kubernetes Charts](https://github.com/kubernetes/charts) team, choose to use only initial lower case letters in order to distinguish local names from those built-in. In this guide, we follow that convention.

