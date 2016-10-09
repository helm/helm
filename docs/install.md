# Installing Helm

There are two parts to Helm: The Helm client (`helm`) and the Helm
server (Tiller). This guide shows how to install the client, and then
proceeds to show two ways to install the server.

## Installing the Helm Client

The Helm client can be installed either from source, or from pre-built binary
releases.

### From the GitHub Releases

Every [release](https://github.com/kubernetes/helm/releases) of Helm
provides binary releases for a variety of OSes. These binary versions
can be manually downloaded and installed.

1. Download your [desired version](https://github.com/kubernetes/helm/releases)
2. Unpack it (`tar -zxvf helm-v2.0.0-linux-amd64.tgz`)
3. Find the `helm` binary in the unpacked directory, and move it to its
   desired destination (`mv linux-amd64/helm /usr/local/bin/helm`)

From there, you should be able to run the client: `helm help`.

### From Homebrew (Mac OSX)

Members of the Kubernetes community have contributed a Helm cask built to
Homebrew. This formula is generally up to date.

```
brew cask install helm
```

(Note: There is also a formula for emacs-helm, which is a different
project.)

### From Canary Builds

"Canary" builds are versions of the Helm software that are built from
the latest master branch. They are not official releases, and may not be
stable. However, they offer the opportunity to test the cutting edge
features.

Canary Helm binaries are stored in the [Kubernetes Helm GCS bucket](http://storage.googleapis.com/kubernetes-helm).
Here are links to the common builds:

- [Linux AMD64](http://storage.googleapis.com/kubernetes-helm/helm-canary-linux-amd64.tar.gz)
- [OSX AMD64](http://storage.googleapis.com/kubernetes-helm/helm-canary-darwin-amd64.tar.gz)
- [Experimental Windows AMD64](http://storage.googleapis.com/kubernetes-helm/helm-canary-windows-amd64.zip)

### From Source (Linux, Mac OSX)

Building Helm from source is slightly more work, but is the best way to
go if you want to test the latest (pre-release) Helm version.

You must have a working Go environment with (https://github.com/Masterminds/glide)[glide]and Mercurial installed.

```console
$ cd $GOPATH
$ mkdir -p src/k8s.io
$ cd src/k8s.io
$ git clone https://github.com/kubernetes/helm.git
$ cd helm
$ make bootstrap build
```

The `bootstrap` target will attempt to install dependencies, rebuild the
`vendor/` tree, and validate configuration.

The `build` target will compile `helm` and place it in `bin/helm`.
Tiller is also compiled, and is placed in `bin/tiller`.

## Installing Tiller

Tiller, the server portion of Helm, typically runs inside of your
Kubernetes cluster. But for development, it can also be run locally, and
configured to talk to a remote Kubernetes cluster.

### Easy In-Cluster Installation

The easiest way to install `tiller` into the cluster is simply to run
`helm init`. This will validate that `helm`'s local environment is set
up correctly (and set it up if necessary). Then it will connect to
whatever cluster `kubectl` connects to by default (`kubectl config
view`). Once it connects, it will install `tiller` into the
`kube-system` namespace.

After `helm init`, you should be able to run `kubectl get po --namespace
kube-system` and see Tiller running.

Once Tiller is installed, running `helm version` should show you both
the client and server version. (If it shows only the client version,
`helm` cannot yet connect to the server. Use `kubectl` to see if any
`tiller` pods are running.)

### Installing Tiller Canary Builds

Canary images are built from the `master` branch. They may not be
stable, but they offer you the chance to test out the latest features.

The easiest way to install a canary image is to use `helm init` with the
`--tiller-image` flag:

```console
$ helm init -i "gcr.io/kubernetes-helm/tiller:canary"
```

This will use the most recently built container image. You can always
uninstall Tiller by deleting the Tiller deployment from the
`kube-system` namespace using `kubectl`.

### Running Tiller Locally

For development, it is sometimes easier to work on Tiller locally, and
configure it to connect to a remote Kubernetes cluster.

The process of building Tiller is explained above.

Once `tiller` has been built, simply start it:

```console
$ bin/tiller
Tiller running on :44134
```

When Tiller is running locally, it will attempt to connect to the
Kubernetes cluster that is configured by `kubectl`. (Run `kubectl config
view` to see which cluster that is.)

You must tell `helm` to connect to this new local Tiller host instead of
connecting to the one in-cluster. There are two ways to do this. The
first is to specify the `--host` option on the command line. The second
is to set the `$HELM_HOST` environment variable.

```console
$ export HELM_HOST=localhost:44134
$ helm version # Should connect to localhost.
Client: &version.Version{SemVer:"v2.0.0-alpha.4", GitCommit:"db...", GitTreeState:"dirty"}
Server: &version.Version{SemVer:"v2.0.0-alpha.4", GitCommit:"a5...", GitTreeState:"dirty"}
```

Importantly, even when running locally, Tiller will store release
configuration in ConfigMaps inside of Kubernetes.

## Conclusion

In most cases, installation is as simple as getting a pre-built `helm` binary
and running `helm init`. This document covers additional cases for those
who want to do more sophisticated things with Helm.

Once you have the Helm Client and Tiller successfully installed, you can
move on to using Helm to manage charts.
