## helm fetch

download a chart from a repository and (optionally) unpack it in local directory

### Synopsis



Retrieve a package from a package repository, and download it locally.

This is useful for fetching packages to inspect, modify, or repackage. It can
also be used to perform cryptographic verification of a chart without installing
the chart.

There are options for unpacking the chart after download. This will create a
directory for the chart and uncompress into that directory.

If the --verify flag is specified, the requested chart MUST have a provenance
file, and MUST pass the verification process. Failure in any part of this will
result in an error, and the chart will not be saved locally.


```
helm fetch [flags] [chart URL | repo/chartname] [...]
```

### Options

```
      --ca-file string       verify certificates of HTTPS-enabled servers using this CA bundle
      --cert-file string     identify HTTPS client using this SSL certificate file
  -d, --destination string   location to write the chart. If this and tardir are specified, tardir is appended to this (default ".")
      --devel                use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --key-file string      identify HTTPS client using this SSL key file
      --keyring string       keyring containing public keys (default "~/.gnupg/pubring.gpg")
      --password string      chart repository password
      --prov                 fetch the provenance file, but don't perform verification
      --repo string          chart repository url where to locate the requested chart
      --untar                if set to true, will untar the chart after downloading it
      --untardir string      if untar is specified, this flag specifies the name of the directory into which the chart is expanded (default ".")
      --username string      chart repository username
      --verify               verify the package against its signature
      --version string       specific version of a chart. Without this, the latest version is fetched
```

### Options inherited from parent commands

```
      --debug                           enable verbose output
      --home string                     location of your Helm config. Overrides $HELM_HOME (default "~/.helm")
      --host string                     address of Tiller. Overrides $HELM_HOST
      --kube-context string             name of the kubeconfig context to use
      --tiller-connection-timeout int   the duration (in seconds) Helm will wait to establish a connection to tiller (default 300)
      --tiller-namespace string         namespace of Tiller (default "kube-system")
```

### SEE ALSO
* [helm](helm.md)	 - The Helm package manager for Kubernetes.
