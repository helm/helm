# Installation: Frequently Asked Questions

This section tracks some of the more frequently encountered issues with installing
or getting started with Helm.

**We'd love your help** making this document better. To add, correct, or remove
information, [file an issue](https://github.com/kubernetes/helm/issues) or
send us a pull request.

## Downloading

I want to know more about my downloading options.

**Q: I can't get to GitHub releases of the newest Helm. Where are they?**

A: We no longer use GitHub releases. Binaries are now stored in a
[GCS public bucket](https://kubernetes-helm.storage.googleapis.com).

**Q: Why aren't there Debian/Fedora/... native packages of Helm?**

We'd love to provide these or point you toward a trusted provider. If you're
interested in helping, we'd love it. This is how the Homebrew formula was
started.

**Q: Why do you provide a `curl ...|bash` script?**

A: There is a script in our repository (`scripts/get`) that can be executed as
a `curl ..|bash` script. The transfers are all protected by HTTPS, and the script
does some auditing of the packages it fetches. However, the script has all the
usual dangers of any shell script.

We provide it because it is useful, but we suggest that users carefully read the
script first. What we'd really like, though, are better packaged releases of
Helm.

## Installing

I'm trying to install Helm/Tiller, but something is not right.

**Q: How do I put the Helm client files somewhere other than ~/.helm?**

Set the `$HELM_HOME` environment variable, and then run `helm init`:

```console
export HELM_HOME=/some/path
helm init --client-only
```

Note that if you have existing repositories, you will need to re-add them
with `helm repo add...`.

**Q: How do I configure Helm, but not install Tiller?**

A: By default, `helm init` will ensure that the local `$HELM_HOME` is configured,
and then install Tiller on your cluster. To locally configure, but not install
Tiller, use `helm init --client-only`.

**Q: How do I manually install Tiller on the cluster?**

A: Tiller is installed as a Kubernetes `deployment`. You can get the manifest
by running `helm init --dry-run --debug`, and then manually install it with
`kubectl`. It is suggested that you do not remove or change the labels on that
deployment, as they are sometimes used by supporting scripts and tools.

**Q: Why do I get `Error response from daemon: target is unknown` during Tiller install?**

A: Users have reported being unable to install Tiller on Kubernetes instances that
are using Docker 1.13.0. The root cause of this was a bug in Docker that made
that one version incompatible with images pushed to the Docker registry by
earlier versions of Docker.

This [issue](https://github.com/docker/docker/issues/30083) was fixed shortly
after the release, and is available in Docker 1.13.1-RC1 and later.

## Getting Started

I successfully installed Helm/Tiller but I can't use it.

**Q: Trying to use Helm, I get the error "client transport was broken"**

```
E1014 02:26:32.885226   16143 portforward.go:329] an error occurred forwarding 37008 -> 44134: error forwarding port 44134 to pod tiller-deploy-2117266891-e4lev_kube-system, uid : unable to do port forwarding: socat not found.
2016/10/14 02:26:32 transport: http2Client.notifyError got notified that the client transport was broken EOF.
Error: transport is closing
```

A: This is usually a good indication that Kubernetes is not set up to allow port forwarding.

Typically, the missing piece is `socat`. If you are running CoreOS, we have been
told that it may have been misconfigured on installation. The CoreOS team
recommends reading this:

- https://coreos.com/kubernetes/docs/latest/kubelet-wrapper.html

Here are a few resolved issues that may help you get started:

- https://github.com/kubernetes/helm/issues/1371
- https://github.com/kubernetes/helm/issues/966

**Q: Trying to use Helm, I get the error "lookup XXXXX on 8.8.8.8:53: no such host"**

```
Error: Error forwarding ports: error upgrading connection: dial tcp: lookup kube-4gb-lon1-02 on 8.8.8.8:53: no such host
```

A: We have seen this issue with Ubuntu and Kubeadm in multi-node clusters. The
issue is that the nodes expect certain DNS records to be obtainable via global
DNS. Until this is resolved upstream, you can work around the issue as
follows:

1) Add entries to `/etc/hosts` on the master mapping your hostnames to their public IPs
2) Install `dnsmasq` on the master (e.g. `apt install -y dnsmasq`)
3) Kill the k8s api server container on master (kubelet will recreate it)
4) Then `systemctl restart docker` (or reboot the master) for it to pick up the /etc/resolv.conf changes

See this issue for more information: https://github.com/kubernetes/helm/issues/1455

**Q: On GKE (Google Container Engine) I get "No SSH tunnels currently open"**

```
Error: Error forwarding ports: error upgrading connection: No SSH tunnels currently open. Were the targets able to accept an ssh-key for user "gke-[redacted]"?
```

Another variation of the error message is:


```
Unable to connect to the server: x509: certificate signed by unknown authority

```

A: The issue is that your local Kubernetes config file must have the correct credentials.

When you create a cluster on GKE, it will give you credentials, including SSL
certificates and certificate authorities. These need to be stored in a Kubernetes
config file (Default: `~/.kube/config` so that `kubectl` and `helm` can access
them.

**Q: When I run a Helm command, I get an error about the tunnel or proxy**

A: Helm uses the Kubernetes proxy service to connect to the Tiller server.
If the command `kubectl proxy` does not work for you, neither will Helm.
Typically, the error is related to a missing `socat` service.

**Q: Tiller crashes with a panic**

When I run a command on Helm, Tiller crashes with an error like this:

```
Tiller is listening on :44134
Probes server is listening on :44135
Storage driver is ConfigMap
Cannot initialize Kubernetes connection: the server has asked for the client to provide credentials 2016-12-20 15:18:40.545739 I | storage.go:37: Getting release "bailing-chinchilla" (v1) from storage
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x8053d5]

goroutine 77 [running]:
panic(0x1abbfc0, 0xc42000a040)
        /usr/local/go/src/runtime/panic.go:500 +0x1a1
k8s.io/helm/vendor/k8s.io/kubernetes/pkg/client/unversioned.(*ConfigMaps).Get(0xc4200c6200, 0xc420536100, 0x15, 0x1ca7431, 0x6, 0xc42016b6a0)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/vendor/k8s.io/kubernetes/pkg/client/unversioned/configmap.go:58 +0x75
k8s.io/helm/pkg/storage/driver.(*ConfigMaps).Get(0xc4201d6190, 0xc420536100, 0x15, 0xc420536100, 0x15, 0xc4205360c0)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/pkg/storage/driver/cfgmaps.go:69 +0x62
k8s.io/helm/pkg/storage.(*Storage).Get(0xc4201d61a0, 0xc4205360c0, 0x12, 0xc400000001, 0x12, 0x0, 0xc420200070)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/pkg/storage/storage.go:38 +0x160
k8s.io/helm/pkg/tiller.(*ReleaseServer).uniqName(0xc42002a000, 0x0, 0x0, 0xc42016b800, 0xd66a13, 0xc42055a040, 0xc420558050, 0xc420122001)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/pkg/tiller/release_server.go:577 +0xd7
k8s.io/helm/pkg/tiller.(*ReleaseServer).prepareRelease(0xc42002a000, 0xc42027c1e0, 0xc42002a001, 0xc42016bad0, 0xc42016ba08)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/pkg/tiller/release_server.go:630 +0x71
k8s.io/helm/pkg/tiller.(*ReleaseServer).InstallRelease(0xc42002a000, 0x7f284c434068, 0xc420250c00, 0xc42027c1e0, 0x0, 0x31a9, 0x31a9)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/pkg/tiller/release_server.go:604 +0x78
k8s.io/helm/pkg/proto/hapi/services._ReleaseService_InstallRelease_Handler(0x1c51f80, 0xc42002a000, 0x7f284c434068, 0xc420250c00, 0xc42027c190, 0x0, 0x0, 0x0, 0x0, 0x0)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/pkg/proto/hapi/services/tiller.pb.go:747 +0x27d
k8s.io/helm/vendor/google.golang.org/grpc.(*Server).processUnaryRPC(0xc4202f3ea0, 0x28610a0, 0xc420078000, 0xc420264690, 0xc420166150, 0x288cbe8, 0xc420250bd0, 0x0, 0x0)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/vendor/google.golang.org/grpc/server.go:608 +0xc50
k8s.io/helm/vendor/google.golang.org/grpc.(*Server).handleStream(0xc4202f3ea0, 0x28610a0, 0xc420078000, 0xc420264690, 0xc420250bd0)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/vendor/google.golang.org/grpc/server.go:766 +0x6b0
k8s.io/helm/vendor/google.golang.org/grpc.(*Server).serveStreams.func1.1(0xc420124710, 0xc4202f3ea0, 0x28610a0, 0xc420078000, 0xc420264690)
        /home/ubuntu/.go_workspace/src/k8s.io/helm/vendor/google.golang.org/grpc/server.go:419 +0xab
created by k8s.io/helm/vendor/google.golang.org/grpc.(*Server).serveStreams.func1
        /home/ubuntu/.go_workspace/src/k8s.io/helm/vendor/google.golang.org/grpc/server.go:420 +0xa3
```

A: Check your security settings for Kubernetes.

A panic in Tiller is almost always the result of a failure to negotiate with the
Kubernetes API server (at which point Tiller can no longer do anything useful, so
it panics and exits).

Often, this is a result of authentication failing because the Pod in which Tiller
is running does not have the right token.

To fix this, you will need to change your Kubernetes configuration. Make sure
that `--service-account-private-key-file` from `controller-manager` and
`--service-account-key-file` from apiserver point to the _same_ x509 RSA key.


## Upgrading

My Helm used to work, then I upgrade. Now it is broken.

**Q: After upgrade, I get the error "Client version is incompatible". What's wrong?**

Tiller and Helm have to negotiate a common version to make sure that they can safely
communicate without breaking API assumptions. That error means that the version
difference is too great to safely continue. Typically, you need to upgrade
Tiller manually for this.

The [Installation Guide](install.md) has definitive information about safely
upgrading Helm and Tiller.

The rules for version numbers are as follows:

- Pre-release versions are incompatible with everything else. `Alpha.1` is incompatible with `Alpha.2`.
- Patch revisions _are compatible_: 1.2.3 is compatible with 1.2.4
- Minor revisions _are not compatible_: 1.2.0 is not compatible with 1.3.0,
  though we may relax this constraint in the future.
- Major revisions _are not compatible_: 1.0.0 is not compatible with 2.0.0.

## Uninstalling

I am trying to remove stuff.

**Q: When I delete the Tiller deployment, how come all the releases are still there?**

Releases are stored in ConfigMaps inside of the `kube-system` namespace. You will
have to manually delete them to get rid of the record.

**Q: I want to delete my local Helm. Where are all its files?**

Along with the `helm` binary, Helm stores some files in `$HELM_HOME`, which is
located by default in `~/.helm`.
