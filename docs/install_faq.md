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

By default, `helm init` will ensure that the local `$HELM_HOME` is configured,
and then install Tiller on your cluster. To locally configure, but not install
Tiller, use `helm init --client-only`.

**Q: How do I manually install Tiller on the cluster?**

Tiller is installed as a Kubernetes `deployment`. You can get the manifest
by running `helm init --dry-run --debug`, and then manually install it with
`kubectl`. It is suggested that you do not remove or change the labels on that
deployment, as they are sometimes used by supporting scripts and tools.


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


## Upgrading

My Helm used to work, then I upgrade. Now it is broken.

**Q: After upgrade, I get the error "Client version is incompatible". What's wrong?**

Tiller and Helm have to negotiate a common version to make sure that they can safely
communicate without breaking API assumptions. That error means that the version
difference is too great to safely continue. Typically, you need to upgrade
Tiller manually for this.

The [Installation Guide](install.yaml) has definitive information about safely
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
