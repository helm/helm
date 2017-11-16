## helm install

install a chart archive

### Synopsis



This command installs a chart archive.

The install argument must be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line.

	$ helm install -f myvalues.yaml ./redis

or

	$ helm install --set name=prod ./redis

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

	$ helm install -f myvalues.yaml -f override.yaml ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

	$ helm install --set foo=bar --set foo=newbar ./redis


To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined. This will still require a
round-trip to the Tiller server.

If --verify is set, the chart MUST have a provenance file, and the provenenace
fall MUST pass all verification steps.

There are four different ways you can express the chart you want to install:

1. By chart reference: helm install stable/mariadb
2. By path to a packaged chart: helm install ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm install ./nginx
4. By absolute URL: helm install https://example.com/charts/nginx-1.2.3.tgz

CHART REFERENCES

A chart reference is a convenient way of reference a chart in a chart repository.

When you use a chart reference ('stable/mariadb'), Helm will look in the local
configuration for a chart repository named 'stable', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest
version of that chart unless you also supply a version number with the
'--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.


```
helm install [CHART]
```

### Options

```
      --ca-file string         verify certificates of HTTPS-enabled servers using this CA bundle
      --cert-file string       identify HTTPS client using this SSL certificate file
      --devel                  use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --dry-run                simulate an install
      --key-file string        identify HTTPS client using this SSL key file
      --keyring string         location of public keys used for verification (default "~/.gnupg/pubring.gpg")
  -n, --name string            release name. If unspecified, it will autogenerate one for you
      --name-template string   specify template used to name the release
      --namespace string       namespace to install the release into. Defaults to the current kube config namespace.
      --no-hooks               prevent hooks from running during install
      --replace                re-use the given name, even if that name is already used. This is unsafe in production
      --repo string            chart repository url where to locate the requested chart
      --set stringArray        set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --timeout int            time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks) (default 300)
      --tls                    enable TLS for request
      --tls-ca-cert string     path to TLS CA certificate file (default "$HELM_HOME/ca.pem")
      --tls-cert string        path to TLS certificate file (default "$HELM_HOME/cert.pem")
      --tls-key string         path to TLS key file (default "$HELM_HOME/key.pem")
      --tls-verify             enable TLS for request and verify remote
  -f, --values valueFiles      specify values in a YAML file or a URL(can specify multiple) (default [])
      --verify                 verify the package before installing it
      --version string         specify the exact chart version to install. If this is not specified, the latest version is installed
      --wait                   if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout
```

### Options inherited from parent commands

```
      --debug                     enable verbose output
      --home string               location of your Helm config. Overrides $HELM_HOME (default "~/.helm")
      --host string               address of Tiller. Overrides $HELM_HOST
      --kube-context string       name of the kubeconfig context to use
      --tiller-namespace string   namespace of Tiller (default "kube-system")
```

### SEE ALSO
* [helm](helm.md)	 - The Helm package manager for Kubernetes.

###### Auto generated by spf13/cobra on 15-Nov-2017
