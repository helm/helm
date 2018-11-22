# Installation: Frequently Asked Questions

This section tracks some of the more frequently encountered issues with installing
or getting started with Helm.

**We'd love your help** making this document better. To add, correct, or remove
information, [file an issue](https://github.com/helm/helm/issues) or
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

I'm trying to install Helm, but something is not right.

**Q: How do I put the Helm client files somewhere other than ~/.helm?**

Set the `$HELM_HOME` environment variable, and then run `helm init`:

```console
export HELM_HOME=/some/path
helm init --client-only
```

Note that if you have existing repositories, you will need to re-add them
with `helm repo add...`.

**Q: How do I configure Helm?**

A: By default, `helm init` will ensure that the local `$HELM_HOME` is configured.


## Getting Started

I successfully installed Helm but I can't use it.

**Q: Trying to use Helm, I get the error "lookup XXXXX on 8.8.8.8:53: no such host"**

```
Error: Error forwarding ports: error upgrading connection: dial tcp: lookup kube-4gb-lon1-02 on 8.8.8.8:53: no such host
```

A: We have seen this issue with Ubuntu and Kubeadm in multi-node clusters. The
issue is that the nodes expect certain DNS records to be obtainable via global
DNS. Until this is resolved upstream, you can work around the issue as
follows. On each of the control plane nodes:

1) Add entries to `/etc/hosts`, mapping your hostnames to their public IPs
2) Install `dnsmasq` (e.g. `apt install -y dnsmasq`)
3) Remove the k8s api server container (kubelet will recreate it)
4) Then `systemctl restart docker` (or reboot the node) for it to pick up the /etc/resolv.conf changes

See this issue for more information: https://github.com/helm/helm/issues/1455

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


## Uninstalling

I am trying to remove stuff.

**Q: I want to delete my local Helm. Where are all its files?**

Along with the `helm` binary, Helm stores some files in `$HELM_HOME`, which is
located by default in `~/.helm`.
