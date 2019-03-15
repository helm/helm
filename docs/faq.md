# Frequently Asked Questions

This page provides help with the most common questions about Helm.

**We'd love your help** making this document better. To add, correct, or remove
information, [file an issue](https://github.com/helm/helm/issues) or
send us a pull request.

## Changes since Helm 2

Here's an exhaustive list of all the major changes introduced in Helm 3.

### Go import path changes

In Helm 3, Helm switched the Go import path over from `k8s.io/helm` to `helm.sh/helm`. If you intend
to upgrade to the Helm 3 Go client libraries, make sure to change your import paths.


## Installing

### Why aren't there Debian/Fedora/... native packages of Helm?

We'd love to provide these or point you toward a trusted provider. If you're
interested in helping, we'd love it. This is how the Homebrew formula was
started.

### Why do you provide a `curl ...|bash` script?

There is a script in our repository (`scripts/get`) that can be executed as
a `curl ..|bash` script. The transfers are all protected by HTTPS, and the script
does some auditing of the packages it fetches. However, the script has all the
usual dangers of any shell script.

We provide it because it is useful, but we suggest that users carefully read the
script first. What we'd really like, though, are better packaged releases of
Helm.

### How do I put the Helm client files somewhere other than ~/.helm?

Set the `$HELM_HOME` environment variable, and then run `helm init`:

```console
export HELM_HOME=/some/path
helm init --client-only
```

Note that if you have existing repositories, you will need to re-add them
with `helm repo add...`.


## Uninstalling

### I want to delete my local Helm. Where are all its files?

Along with the `helm` binary, Helm stores some files in `$HELM_HOME`, which is
located by default in `~/.helm`.


## Troubleshooting

### On GKE (Google Container Engine) I get "No SSH tunnels currently open"

```
Error: Error forwarding ports: error upgrading connection: No SSH tunnels currently open. Were the targets able to accept an ssh-key for user "gke-[redacted]"?
```

Another variation of the error message is:


```
Unable to connect to the server: x509: certificate signed by unknown authority

```

The issue is that your local Kubernetes config file must have the correct credentials.

When you create a cluster on GKE, it will give you credentials, including SSL
certificates and certificate authorities. These need to be stored in a Kubernetes
config file (Default: `~/.kube/config` so that `kubectl` and `helm` can access
them.

### Why do I get a `unsupported protocol scheme ""` error when trying to pull a chart from my custom repo?**

(Helm < 2.5.0) This is likely caused by you creating your chart repo index without specifying the `--url` flag.
Try recreating your `index.yaml` file with a command like `helm repo index --url http://my-repo/charts .`,
and then re-uploading it to your custom charts repo.

This behavior was changed in Helm 2.5.0.
