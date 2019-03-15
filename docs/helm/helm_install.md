## helm install

install a chart

### Synopsis


This command installs a chart archive.

The install argument must be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force
a string value use '--set-string'.

	$ helm install -f myvalues.yaml myredis ./redis

or

	$ helm install --set name=prod myredis ./redis

or

	$ helm install --set-string long_int=1234567890 myredis ./redis

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

	$ helm install -f myvalues.yaml -f override.yaml  myredis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

	$ helm install --set foo=bar --set foo=newbar  myredis ./redis


To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined. This will still require a
round-trip to the Tiller server.

If --verify is set, the chart MUST have a provenance file, and the provenance
file MUST pass all verification steps.

There are five different ways you can express the chart you want to install:

1. By chart reference: helm install stable/mariadb
2. By path to a packaged chart: helm install ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm install ./nginx
4. By absolute URL: helm install https://example.com/charts/nginx-1.2.3.tgz
5. By chart reference and repo url: helm install --repo https://example.com/charts/ nginx

CHART REFERENCES

A chart reference is a convenient way of reference a chart in a chart repository.

When you use a chart reference with a repo prefix ('stable/mariadb'), Helm will look in the local
configuration for a chart repository named 'stable', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest
version of that chart unless you also supply a version number with the
'--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.


```
helm install [NAME] [CHART] [flags]
```

### Options

```
      --ca-file string           verify certificates of HTTPS-enabled servers using this CA bundle
      --cert-file string         identify HTTPS client using this SSL certificate file
      --dependency-update        run helm dependency update before installing the chart
      --devel                    use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --dry-run                  simulate an install
  -g, --generate-name            generate the name (and omit the NAME parameter)
  -h, --help                     help for install
      --key-file string          identify HTTPS client using this SSL key file
      --keyring string           location of public keys used for verification (default "~/.gnupg/pubring.gpg")
      --name-template string     specify template used to name the release
      --no-hooks                 prevent hooks from running during install
      --password string          chart repository password where to locate the requested chart
      --replace                  re-use the given name, even if that name is already used. This is unsafe in production
      --repo string              chart repository url where to locate the requested chart
      --set stringArray          set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-string stringArray   set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --timeout int              time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks) (default 300)
      --username string          chart repository username where to locate the requested chart
  -f, --values strings           specify values in a YAML file or a URL(can specify multiple)
      --verify                   verify the package before installing it
      --version string           specify the exact chart version to install. If this is not specified, the latest version is installed
      --wait                     if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout
```

### Options inherited from parent commands

```
      --debug                 enable verbose output
      --home string           location of your Helm config. Overrides $HELM_HOME (default "~/.helm")
      --kube-context string   name of the kubeconfig context to use
      --kubeconfig string     path to the kubeconfig file
  -n, --namespace string      namespace scope for this request
```

### SEE ALSO

* [helm](helm.md)	 - The Helm package manager for Kubernetes.

###### Auto generated by spf13/cobra on 15-Mar-2019
