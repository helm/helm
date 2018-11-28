# Developers Guide

This guide explains how to set up your environment for developing on
Helm.

## Prerequisites

- The latest version of Go
- The latest version of Dep
- A Kubernetes cluster w/ kubectl (optional)
- Git

## Building Helm

We use Make to build our programs. The simplest way to get started is:

```console
$ make bootstrap build
```

NOTE: This will fail if not running from the path `$GOPATH/src/k8s.io/helm`. The
directory `k8s.io` should not be a symlink or `build` will not find the relevant
packages.

This will build both Helm and the Helm library. `make bootstrap` will attempt to
install certain tools if they are missing.

To run all the tests (without running the tests for `vendor/`), run
`make test`.

To run Helm locally, you can run `bin/helm`.

- Helm is known to run on macOS and most Linuxes, including Alpine.

### Man pages

Man pages and Markdown documentation are already pre-built in `docs/`. You may
regenerate documentation using `make docs`.

To expose the Helm man pages to your `man` client, you can put the files in your
`$MANPATH`:

```
$ export MANPATH=$GOPATH/src/k8s.io/helm/docs/man:$MANPATH
$ man helm
```


## Docker Images

To build Docker images, use `make docker-build`.

Pre-build images are already available in the official Kubernetes Helm
GCR registry.

## Running a Local Cluster

For development, we highly recommend using the
[Kubernetes Minikube](https://github.com/kubernetes/minikube)
developer-oriented distribution.

## Contribution Guidelines

We welcome contributions. This project has set up some guidelines in
order to ensure that (a) code quality remains high, (b) the project
remains consistent, and (c) contributions follow the open source legal
requirements. Our intent is not to burden contributors, but to build
elegant and high-quality open source code so that our users will benefit.

Make sure you have read and understood the main CONTRIBUTING guide:

https://github.com/helm/helm/blob/master/CONTRIBUTING.md

### Structure of the Code

The code for the Helm project is organized as follows:

- The individual programs are located in `cmd/`. Code inside of `cmd/`
  is not designed for library re-use.
- Shared libraries are stored in `pkg/`.
- The raw ProtoBuf files are stored in `_proto/hapi` (where `hapi` stands for
  the Helm Application Programming Interface).
- The Go files generated from the `proto` definitions are stored in `pkg/proto`.
- The `scripts/` directory contains a number of utility scripts. Most of these
  are used by the CI/CD pipeline.
- The `rootfs/` folder is used for Docker-specific files.
- The `docs/` folder is used for documentation and examples.

Go dependencies are managed with
[Dep](https://github.com/golang/dep) and stored in the
`vendor/` directory.

### Git Conventions

We use Git for our version control system. The `master` branch is the
home of the current development candidate. Releases are tagged.

We accept changes to the code via GitHub Pull Requests (PRs). One
workflow for doing this is as follows:

1. Go to your `$GOPATH/src/k8s.io` directory and `git clone` the
   `github.com/kubernetes/helm` repository.
2. Fork that repository into your GitHub account
3. Add your repository as a remote for `$GOPATH/src/k8s.io/helm`
4. Create a new working branch (`git checkout -b feat/my-feature`) and
   do your work on that branch.
5. When you are ready for us to review, push your branch to GitHub, and
   then open a new pull request with us.

For Git commit messages, we follow the [Semantic Commit Messages](http://karma-runner.github.io/0.13/dev/git-commit-msg.html):

```
fix(helm): add --foo flag to 'helm install'

When 'helm install --foo bar' is run, this will print "foo" in the
output regardless of the outcome of the installation.

Closes #1234
```

Common commit types:

- fix: Fix a bug or error
- feat: Add a new feature
- docs: Change documentation
- test: Improve testing
- ref: refactor existing code

Common scopes:

- helm: The Helm CLI
- pkg/lint: The lint package. Follow a similar convention for any
  package
- `*`: two or more scopes

Read more:
- The [Deis Guidelines](https://github.com/deis/workflow/blob/master/src/contributing/submitting-a-pull-request.md)
  were the inspiration for this section.
- Karma Runner [defines](http://karma-runner.github.io/0.13/dev/git-commit-msg.html) the semantic commit message idea.

### Go Conventions

We follow the Go coding style standards very closely. Typically, running
`go fmt` will make your code beautiful for you.

We also typically follow the conventions recommended by `go lint` and
`gometalinter`. Run `make test-style` to test the style conformance.

Read more:

- Effective Go [introduces formatting](https://golang.org/doc/effective_go.html#formatting).
- The Go Wiki has a great article on [formatting](https://github.com/golang/go/wiki/CodeReviewComments).

### Protobuf Conventions

Because this project is largely Go code, we format our Protobuf files as
closely to Go as possible. There are currently no real formatting rules
or guidelines for Protobuf, but as they emerge, we may opt to follow
those instead.

Standards:
- Tabs for indentation, not spaces.
- Spacing rules follow Go conventions (curly braces at line end, spaces
  around operators).

Conventions:
- Files should specify their package with `option go_package = "...";`
- Comments should translate into good Go code comments (since `protoc`
  copies comments into the destination source code file).
- RPC functions are defined in the same file as their request/response
  messages.
- Deprecated RPCs, messages, and fields are marked deprecated in the comments (`// UpdateFoo
  DEPRECATED updates a foo.`).
