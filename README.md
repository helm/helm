# Kubernetes Helm

[![CircleCI](https://circleci.com/gh/kubernetes/helm.svg?style=svg)](https://circleci.com/gh/kubernetes/helm)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes/helm)](https://goreportcard.com/report/github.com/kubernetes/helm)

Helm is a tool for managing Kubernetes charts. Charts are packages of
pre-configured Kubernetes resources.

Use Helm to...

- Find and use popular software packaged as Kubernetes charts
- Share your own applications as Kubernetes charts
- Create reproducible builds of your Kubernetes applications
- Intelligently manage your Kubernetes manifest files
- Manage releases of Helm packages

## Helm in a Handbasket

Helm is a tool that streamlines installing and managing Kubernetes applications.
Think of it like apt/yum/homebrew for Kubernetes.

- Helm has two parts: a client (`helm`) and a server (`tiller`)
- Tiller runs inside of your Kubernetes cluster, and manages releases (installations)
  of your charts.
- Helm runs on your laptop, CI/CD, or wherever you want it to run.
- Charts are Helm packages that contain at least two things:
  - A description of the package (`Chart.yaml`)
  - One or more templates, which contain Kubernetes manifest files
- Charts can be stored on disk, or fetched from remote chart repositories
  (like Debian or RedHat packages)

## Install

Binary downloads of the Helm client can be found at the following links:

- [OSX](https://kubernetes-helm.storage.googleapis.com/helm-v2.2.0-darwin-amd64.tar.gz)
- [Linux](https://kubernetes-helm.storage.googleapis.com/helm-v2.2.0-linux-amd64.tar.gz)
- [Linux 32-bit](https://kubernetes-helm.storage.googleapis.com/helm-v2.2.0-linux-386.tar.gz)

Unpack the `helm` binary and add it to your PATH and you are good to go!
macOS/[homebrew](https://brew.sh/) users can also use `brew install kubernetes-helm`.

To rapidly get Helm up and running, start with the [Quick Start Guide](docs/quickstart.md).

See the [installation guide](docs/install.md) for more options,
including installing pre-releases.


## Docs

- [Quick Start](docs/quickstart.md) - Read me first!
- [Installing Helm](docs/install.md) - Install Helm and Tiller
  - [Kubernetes Distribution Notes](docs/kubernetes_distros.md)
  - [Frequently Asked Questions](docs/install_faq.md)
- [Using Helm](docs/using_helm.md) - Learn the Helm tools
  - [Plugins](docs/plugins.md)
- [Developing Charts](docs/charts.md) - An introduction to chart development
	- [Chart Lifecycle Hooks](docs/charts_hooks.md)
	- [Chart Tips and Tricks](docs/charts_tips_and_tricks.md)
	- [Chart Repository Guide](docs/chart_repository.md)
	- [Syncing your Chart Repository](docs/chart_repository_sync_example.md)
	- [Signing Charts](docs/provenance.md)
	- [Writing Tests for Charts](docs/chart_tests.md)
- [Chart Template Developer's Guide](docs/chart_template_guide/index.md) - Master Helm templates
  - [Getting Started with Templates](docs/chart_template_guide/getting_started.md)
  - [Built-in Objects](docs/chart_template_guide/builtin_objects.md)
  - [Values Files](docs/chart_template_guide/values_files.md)
  - [Functions and Pipelines](docs/chart_template_guide/functions_and_pipelines.md)
  - [Flow Control (if/else, with, range, whitespace management)](docs/chart_template_guide/control_structures.md)
  - [Variables](docs/chart_template_guide/variables.md)
  - [Named Templates (Partials)](docs/chart_template_guide/named_templates.md)
  - [Accessing Files Inside Templates](docs/chart_template_guide/accessing_files.md)
  - [Creating a NOTES.txt File](docs/chart_template_guide/notes_files.md)
  - [Subcharts and Global Values](docs/chart_template_guide/subcharts_and_globals.md)
  - [Debugging Templates](docs/chart_template_guide/debugging.md)
  - [Wrapping Up](docs/chart_template_guide/wrapping_up.md)
  - [Appendix A: YAML Techniques](docs/chart_template_guide/yaml_techniques.md)
  - [Appendix B: Go Data Types](docs/chart_template_guide/data_types.md)
- [Related Projects](docs/related.md) - More Helm tools, articles, and plugins
- [Architecture](docs/architecture.md) - Overview of the Helm/Tiller design
- [Developers](docs/developers.md) - About the developers
- [History](docs/history.md) - A brief history of the project
- [Glossary](docs/glossary.md) - Decode the Helm vocabulary

## Roadmap

The [Helm roadmap is currently located on the wiki](https://github.com/kubernetes/helm/wiki/Roadmap).

## Community, discussion, contribution, and support

You can reach the Helm community and developers via the following channels:

- [Kubernetes Slack](https://slack.k8s.io): #helm
- Mailing List: https://groups.google.com/forum/#!forum/kubernetes-sig-apps
- Developer Call: Thursdays at 9:30-10:00 Pacific. https://engineyard.zoom.us/j/366425549

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
