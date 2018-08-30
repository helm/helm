## helm upgrade

upgrade a release

### Synopsis


This command upgrades a release to a specified version of a chart and/or updates chart values.

Required arguments are release and chart. The chart argument can be one of:
 - a chart reference('stable/mariadb'); use '--version' and '--devel' flags for versions other than latest,
 - a path to a chart directory,
 - a packaged chart,
 - a fully qualified URL.

To customize the chart values, use any of
 - '--values'/'-f' to pass in a yaml file holding settings,
 - '--set' to provide one or more key=val pairs directly,
 - '--set-string' to provide key=val forcing val to be stored as a string,
 - '--set-file' to provide key=path to read a single large value from a file at path.

To edit or append to the existing customized values, add the 
 '--reuse-values' flag, otherwise any existing customized values are ignored.

If no chart value arguments are provided on the command line, any existing customized values are carried
forward. If you want to revert to just the values provided in the chart, use the '--reset-values' flag.

You can specify any of the chart value flags multiple times. The priority will be given to the last
(right-most) value specified. For example, if both myvalues.yaml and override.yaml contained a key
called 'Test', the value set in override.yaml would take precedence:

	$ helm upgrade -f myvalues.yaml -f override.yaml redis ./redis

Note that the key name provided to the '--set', '--set-string' and '--set-file' flags can reference
structure elements. Examples:
  - mybool=TRUE
  - livenessProbe.timeoutSeconds=10
  - metrics.annotations[0]=hey,metrics.annotations[1]=ho

which sets the top level key mybool to true, the nested timeoutSeconds to 10, and two array values, respectively.

Note that the value side of the key=val provided to '--set' and '--set-string' flags will pass through
shell evaluation followed by yaml type parsing to produce the final value. This may alter inputs with
special characters in unexpected ways, for example

	$ helm upgrade --set pwd=3jk$o2,z=f\30.e redis ./redis

results in "pwd: 3jk" and "z: f30.e". Use single quotes to avoid shell evaluation and argument delimiters,
and use backslash to escape yaml special characters:

	$ helm upgrade --set pwd='3jk$o2z=f\\30.e' redis ./redis

which results in the expected "pwd: 3jk$o2z=f\30.e". If a single quote occurs in your value then follow
your shell convention for escaping it; for example in bash:

	$ helm upgrade --set pwd='3jk$o2z=f\\30with'\''quote'

which results in "pwd: 3jk$o2z=f\30with'quote".


```
helm upgrade [RELEASE] [CHART] [flags]
```

### Options

```
      --ca-file string           verify certificates of HTTPS-enabled servers using this CA bundle
      --cert-file string         identify HTTPS client using this SSL certificate file
      --description string       specify the description to use for the upgrade, rather than the default
      --devel                    use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --dry-run                  simulate an upgrade
      --force                    force resource update through delete/recreate if needed
  -h, --help                     help for upgrade
  -i, --install                  if a release by this name doesn't already exist, run an install
      --key-file string          identify HTTPS client using this SSL key file
      --keyring string           path to the keyring that contains public signing keys (default "~/.gnupg/pubring.gpg")
      --namespace string         namespace to install the release into (only used if --install is set). Defaults to the current kube config namespace
      --no-hooks                 disable pre/post upgrade hooks
      --password string          chart repository password where to locate the requested chart
      --recreate-pods            performs pods restart for the resource if applicable
      --repo string              chart repository url where to locate the requested chart
      --reset-values             when upgrading, reset the values to the ones built into the chart
      --reuse-values             when upgrading, reuse the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' is specified, this is ignored.
      --set stringArray          set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray     set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-string stringArray   set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --timeout int              time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks) (default 300)
      --tls                      enable TLS for request
      --tls-ca-cert string       path to TLS CA certificate file (default "$HELM_HOME/ca.pem")
      --tls-cert string          path to TLS certificate file (default "$HELM_HOME/cert.pem")
      --tls-hostname string      the server name used to verify the hostname on the returned certificates from the server
      --tls-key string           path to TLS key file (default "$HELM_HOME/key.pem")
      --tls-verify               enable TLS for request and verify remote
      --username string          chart repository username where to locate the requested chart
  -f, --values valueFiles        specify values in a YAML file or a URL(can specify multiple) (default [])
      --verify                   verify the provenance of the chart before upgrading
      --version string           specify the exact chart version to use. If this is not specified, the latest version is used
      --wait                     if set, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful. It will wait for as long as --timeout
```

### Options inherited from parent commands

```
      --debug                           enable verbose output
      --home string                     location of your Helm config. Overrides $HELM_HOME (default "~/.helm")
      --host string                     address of Tiller. Overrides $HELM_HOST
      --kube-context string             name of the kubeconfig context to use
      --kubeconfig string               absolute path to the kubeconfig file to use
      --tiller-connection-timeout int   the duration (in seconds) Helm will wait to establish a connection to tiller (default 300)
      --tiller-namespace string         namespace of Tiller (default "kube-system")
```

### SEE ALSO

* [helm](helm.md)	 - The Helm package manager for Kubernetes.

###### Auto generated by spf13/cobra on 24-Aug-2018
