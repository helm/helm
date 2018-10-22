# Using Helm

This guide explains the basics of using Helm (and Tiller) to manage
packages on your Kubernetes cluster. It assumes that you have already
[installed](install.md) the Helm client and the Tiller server (typically by `helm
init`).

If you are simply interested in running a few quick commands, you may
wish to begin with the [Quickstart Guide](quickstart.md). This chapter
covers the particulars of Helm commands, and explains how to use Helm.

## Three Big Concepts

A *Chart* is a Helm package. It contains all of the resource definitions
necessary to run an application, tool, or service inside of a Kubernetes
cluster. Think of it like the Kubernetes equivalent of a Homebrew formula,
an Apt dpkg, or a Yum RPM file.

A *Repository* is the place where charts can be collected and shared.
It's like Perl's [CPAN archive](http://www.cpan.org) or the
[Fedora Package Database](https://admin.fedoraproject.org/pkgdb/), but for
Kubernetes packages.

A *Release* is an instance of a chart running in a Kubernetes cluster.
One chart can often be installed many times into the same cluster. And
each time it is installed, a new _release_ is created. Consider a MySQL
chart. If you want two databases running in your cluster, you can
install that chart twice. Each one will have its own _release_, which
will in turn have its own _release name_.

With these concepts in mind, we can now explain Helm like this:

Helm installs _charts_ into Kubernetes, creating a new _release_ for
each installation. And to find new charts, you can search Helm chart
_repositories_.

## 'helm search': Finding Charts

When you first install Helm, it is preconfigured to talk to the official
Kubernetes charts repository. This repository contains a number of
carefully curated and maintained charts. This chart repository is named
`stable` by default.

You can see which charts are available by running `helm search`:

```console
$ helm search
NAME                 	VERSION 	DESCRIPTION
stable/drupal   	0.3.2   	One of the most versatile open source content m...
stable/jenkins  	0.1.0   	A Jenkins Helm chart for Kubernetes.
stable/mariadb  	0.5.1   	Chart for MariaDB
stable/mysql    	0.1.0   	Chart for MySQL
...
```

With no filter, `helm search` shows you all of the available charts. You
can narrow down your results by searching with a filter:

```console
$ helm search mysql
NAME               	VERSION	DESCRIPTION
stable/mysql  	0.1.0  	Chart for MySQL
stable/mariadb	0.5.1  	Chart for MariaDB
```

Now you will only see the results that match your filter. 

Why is
`mariadb` in the list? Because its package description relates it to
MySQL. We can use `helm inspect chart` to see this:

```console
$ helm inspect stable/mariadb
Fetched stable/mariadb to mariadb-0.5.1.tgz
description: Chart for MariaDB
engine: gotpl
home: https://mariadb.org
keywords:
- mariadb
- mysql
- database
- sql
...
```

Search is a good way to find available packages. Once you have found a
package you want to install, you can use `helm install` to install it.

## 'helm install': Installing a Package

To install a new package, use the `helm install` command. At its
simplest, it takes only one argument: The name of the chart.

```console
$ helm install stable/mariadb
Fetched stable/mariadb-0.3.0 to /Users/mattbutcher/Code/Go/src/k8s.io/helm/mariadb-0.3.0.tgz
NAME: happy-panda
LAST DEPLOYED: Fri Oct 19 12:40:03 2018
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
==> v1/Pod(related)
NAME                                READY  STATUS   RESTARTS  AGE
happy-panda-mariadb-master-0  0/1    Pending  0         0s
happy-panda-mariadb-slave-0   0/1    Pending  0         0s

==> v1/Secret

NAME                       AGE
happy-panda-mariadb  0s

==> v1/ConfigMap
happy-panda-mariadb-master  0s
happy-panda-mariadb-slave   0s
happy-panda-mariadb-tests   0s

==> v1/Service
happy-panda-mariadb        0s
happy-panda-mariadb-slave  0s

==> v1beta1/StatefulSet
happy-panda-mariadb-master  0s
happy-panda-mariadb-slave   0s


NOTES:

Please be patient while the chart is being deployed

Tip:

  Watch the deployment status using the command: kubectl get pods -w --namespace default -l release=happy-panda

Services:

  echo Master: happy-panda-mariadb.default.svc.cluster.local:3306
  echo Slave:  happy-panda-mariadb-slave.default.svc.cluster.local:3306

Administrator credentials:

  Username: root
  Password : $(kubectl get secret --namespace default happy-panda-mariadb -o jsonpath="{.data.mariadb-root-password}" | base64 --decode)

To connect to your database:

  1. Run a pod that you can use as a client:

      kubectl run happy-panda-mariadb-client --rm --tty -i --image  docker.io/bitnami/mariadb:10.1.36 --namespace default --command -- bash

  2. To connect to master service (read/write):

      mysql -h happy-panda-mariadb.default.svc.cluster.local -uroot -p my_database

  3. To connect to slave service (read-only):

      mysql -h happy-panda-mariadb-slave.default.svc.cluster.local -uroot -p my_database

To upgrade this helm chart:

  1. Obtain the password as described on the 'Administrator credentials' section and set the 'rootUser.password' parameter as shown below:

      ROOT_PASSWORD=$(kubectl get secret --namespace default happy-panda-mariadb -o jsonpath="{.data.mariadb-root-password}" | base64 --decode)
      helm upgrade happy-panda stable/mariadb --set rootUser.password=$ROOT_PASSWORD


```

Now the `mariadb` chart is installed. Note that installing a chart
creates a new _release_ object. The release above is named
`happy-panda`. (If you want to use your own release name, simply use the
`--name` flag on `helm install`.)

During installation, the `helm` client will print useful information
about which resources were created, what the state of the release is,
and also whether there are additional configuration steps you can or
should take.

Helm does not wait until all of the resources are running before it
exits. Many charts require Docker images that are over 600M in size, and
may take a long time to install into the cluster.

To keep track of a release's state, or to re-read configuration
information, you can use `helm status`:

```console
helm status happy-panda
LAST DEPLOYED: Fri Oct 19 12:40:03 2018
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
==> v1/Pod(related)
NAME                                READY  STATUS   RESTARTS  AGE
happy-panda-mariadb-master-0  1/1    Running  0         3m
happy-panda-mariadb-slave-0   1/1    Running  0         3m

==> v1/Secret

NAME                       AGE
happy-panda-mariadb  3m

==> v1/ConfigMap
happy-panda-mariadb-master  3m
happy-panda-mariadb-slave   3m
happy-panda-mariadb-tests   3m

==> v1/Service
happy-panda-mariadb        3m
happy-panda-mariadb-slave  3m

==> v1beta1/StatefulSet
happy-panda-mariadb-master  3m
happy-panda-mariadb-slave   3m


NOTES:

Please be patient while the chart is being deployed

Tip:

  Watch the deployment status using the command: kubectl get pods -w --namespace default -l release=happy-panda

Services:

  echo Master: happy-panda-mariadb.default.svc.cluster.local:3306
  echo Slave:  happy-panda-mariadb-slave.default.svc.cluster.local:3306

Administrator credentials:

  Username: root
  Password : $(kubectl get secret --namespace default happy-panda-mariadb -o jsonpath="{.data.mariadb-root-password}" | base64 --decode)

To connect to your database:

  1. Run a pod that you can use as a client:

      kubectl run happy-panda-mariadb-client --rm --tty -i --image  docker.io/bitnami/mariadb:10.1.36 --namespace default --command -- bash

  2. To connect to master service (read/write):

      mysql -h happy-panda-mariadb.default.svc.cluster.local -uroot -p my_database

  3. To connect to slave service (read-only):

      mysql -h happy-panda-mariadb-slave.default.svc.cluster.local -uroot -p my_database

To upgrade this helm chart:

  1. Obtain the password as described on the 'Administrator credentials' section and set the 'rootUser.password' parameter as shown below:

      ROOT_PASSWORD=$(kubectl get secret --namespace default happy-panda-mariadb -o jsonpath="{.data.mariadb-root-password}" | base64 --decode)
      helm upgrade happy-panda stable/mariadb --set rootUser.password=$ROOT_PASSWORD
```

The above shows the current state of your release.

### Customizing the Chart Before Installing

Installing the way we have here will only use the default configuration
options for this chart. Many times, you will want to customize the chart
to use your preferred configuration.

To see what options are configurable on a chart, use `helm inspect
values`:

```console
helm inspect values stable/mariadb
## Global Docker image registry
## Please, note that this will override the image registry for all the images, including dependencies, configured to use the global value
##
# global:
#   imageRegistry:

## Bitnami MariaDB image
## ref: https://hub.docker.com/r/bitnami/mariadb/tags/
##
image:
  registry: docker.io
  repository: bitnami/mariadb
  tag: 10.1.36
  ## Specify a imagePullPolicy
  ## Defaults to 'Always' if image tag is 'latest', else set to 'IfNotPresent'
  ## ref: http://kubernetes.io/docs/user-guide/images/#pre-pulling-images
  ##
  pullPolicy: IfNotPresent
  ## Optionally specify an array of imagePullSecrets.
  ## Secrets must be manually created in the namespace.
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
  ##
  # pullSecrets:
  #   - myRegistrKeySecretName

service:
  ## Kubernetes service type, ClusterIP and NodePort are supported at present
  type: ClusterIP
  # clusterIp: None
  port: 3306
  ## Specify the nodePort value for the LoadBalancer and NodePort service types.
  ## ref: https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport
  ##
  # nodePort:
  #   master: 30001
  #   slave: 30002

## Pod Security Context
## ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
##
securityContext:
  enabled: true
  fsGroup: 1001
  runAsUser: 1001

rootUser:
  ## MariaDB admin password
  ## ref: https://github.com/bitnami/bitnami-docker-mariadb#setting-the-root-password-on-first-run
  ##
  password:
  ## Use existing secret (ignores root, db and replication passwords)
  # existingSecret:
  ##
  ## Option to force users to specify a password. That is required for 'helm upgrade' to work properly.
  ## If it is not force, a random password will be generated.
  forcePassword: false

db:
  ## MariaDB username and password
  ## ref: https://github.com/bitnami/bitnami-docker-mariadb#creating-a-database-user-on-first-run
  ##
  user:
  password:
  ## Password is ignored if existingSecret is specified.
  ## Database to create
  ## ref: https://github.com/bitnami/bitnami-docker-mariadb#creating-a-database-on-first-run
  ##
  name: my_database
  ## Option to force users to specify a password. That is required for 'helm upgrade' to work properly.
  ## If it is not force, a random password will be generated.
  forcePassword: false

replication:
  ## Enable replication. This enables the creation of replicas of MariaDB. If false, only a
  ## master deployment would be created
  enabled: true
  ##
  ## MariaDB replication user
  ## ref: https://github.com/bitnami/bitnami-docker-mariadb#setting-up-a-replication-cluster
  ##
  user: replicator
  ## MariaDB replication user password
  ## ref: https://github.com/bitnami/bitnami-docker-mariadb#setting-up-a-replication-cluster
  ##
  password:
  ## Password is ignored if existingSecret is specified.
  ##
  ## Option to force users to specify a password. That is required for 'helm upgrade' to work properly.
  ## If it is not force, a random password will be generated.
  forcePassword: false

master:
  ## Mariadb Master additional pod annotations
  ## ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
  # annotations:
  #   - key: key1
  #     value: value1

  ## Affinity for pod assignment
  ## Ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity
  ##
  affinity: {}

  ## Kept for backwards compatibility. You can now disable it by removing it.
  ## if you wish to set it through master.affinity.podAntiAffinity instead.
  ##
  antiAffinity: soft

  ## Tolerations for pod assignment
  ## Ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
  ##
  tolerations: []

  ## Enable persistence using Persistent Volume Claims
  ## ref: http://kubernetes.io/docs/user-guide/persistent-volumes/
  ##
  persistence:
    ## If true, use a Persistent Volume Claim, If false, use emptyDir
    ##
    enabled: true
    # Enable persistence using an existing PVC
    # existingClaim:
    # mountPath:
    ## Persistent Volume Storage Class
    ## If defined, storageClassName: <storageClass>
    ## If set to "-", storageClassName: "", which disables dynamic provisioning
    ## If undefined (the default) or set to null, no storageClassName spec is
    ##   set, choosing the default provisioner.  (gp2 on AWS, standard on
    ##   GKE, AWS & OpenStack)
    ##
    # storageClass: "-"
    ## Persistent Volume Claim annotations
    ##
    annotations:
    ## Persistent Volume Access Mode
    ##
    accessModes:
    - ReadWriteOnce
    ## Persistent Volume size
    ##
    size: 8Gi
    ##

  ## Configure MySQL with a custom my.cnf file
  ## ref: https://mysql.com/kb/en/mysql/configuring-mysql-with-mycnf/#example-of-configuration-file
  ##
  config: |-
    [mysqld]
    skip-name-resolve
    explicit_defaults_for_timestamp
    basedir=/opt/bitnami/mariadb
    port=3306
    socket=/opt/bitnami/mariadb/tmp/mysql.sock
    tmpdir=/opt/bitnami/mariadb/tmp
    max_allowed_packet=16M
    bind-address=0.0.0.0
    pid-file=/opt/bitnami/mariadb/tmp/mysqld.pid
    log-error=/opt/bitnami/mariadb/logs/mysqld.log
    character-set-server=UTF8
    collation-server=utf8_general_ci

    [client]
    port=3306
    socket=/opt/bitnami/mariadb/tmp/mysql.sock
    default-character-set=UTF8

    [manager]
    port=3306
    socket=/opt/bitnami/mariadb/tmp/mysql.sock
    pid-file=/opt/bitnami/mariadb/tmp/mysqld.pid

  ## Configure master resource requests and limits
  ## ref: http://kubernetes.io/docs/user-guide/compute-resources/
  ##
  resources: {}
  livenessProbe:
    enabled: true
    ##
    ## Initializing the database could take some time
    initialDelaySeconds: 120
    ##
    ## Default Kubernetes values
    periodSeconds: 10
    timeoutSeconds: 1
    successThreshold: 1
    failureThreshold: 3
  readinessProbe:
    enabled: true
    initialDelaySeconds: 30
    ##
    ## Default Kubernetes values
    periodSeconds: 10
    timeoutSeconds: 1
    successThreshold: 1
    failureThreshold: 3

slave:
  replicas: 1

  ## Mariadb Slave additional pod annotations
  ## ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
  # annotations:
  #   - key: key1
  #     value: value1

  ## Affinity for pod assignment
  ## Ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity
  ##
  affinity: {}

  ## Kept for backwards compatibility. You can now disable it by removing it.
  ## if you wish to set it through slave.affinity.podAntiAffinity instead.
  ##
  antiAffinity: soft

  ## Tolerations for pod assignment
  ## Ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
  ##
  tolerations: []

  persistence:
    ## If true, use a Persistent Volume Claim, If false, use emptyDir
    ##
    enabled: true
    # storageClass: "-"
    annotations:
    accessModes:
    - ReadWriteOnce
    ## Persistent Volume size
    ##
    size: 8Gi
    ##

  ## Configure MySQL slave with a custom my.cnf file
  ## ref: https://mysql.com/kb/en/mysql/configuring-mysql-with-mycnf/#example-of-configuration-file
  ##
  config: |-
    [mysqld]
    skip-name-resolve
    explicit_defaults_for_timestamp
    basedir=/opt/bitnami/mariadb
    port=3306
    socket=/opt/bitnami/mariadb/tmp/mysql.sock
    tmpdir=/opt/bitnami/mariadb/tmp
    max_allowed_packet=16M
    bind-address=0.0.0.0
    pid-file=/opt/bitnami/mariadb/tmp/mysqld.pid
    log-error=/opt/bitnami/mariadb/logs/mysqld.log
    character-set-server=UTF8
    collation-server=utf8_general_ci

    [client]
    port=3306
    socket=/opt/bitnami/mariadb/tmp/mysql.sock
    default-character-set=UTF8

    [manager]
    port=3306
    socket=/opt/bitnami/mariadb/tmp/mysql.sock
    pid-file=/opt/bitnami/mariadb/tmp/mysqld.pid

  ##
  ## Configure slave resource requests and limits
  ## ref: http://kubernetes.io/docs/user-guide/compute-resources/
  ##
  resources: {}
  livenessProbe:
    enabled: true
    ##
    ## Initializing the database could take some time
    initialDelaySeconds: 120
    ##
    ## Default Kubernetes values
    periodSeconds: 10
    timeoutSeconds: 1
    successThreshold: 1
    failureThreshold: 3
  readinessProbe:
    enabled: true
    initialDelaySeconds: 45
    ##
    ## Default Kubernetes values
    periodSeconds: 10
    timeoutSeconds: 1
    successThreshold: 1
    failureThreshold: 3

metrics:
  enabled: false
  image:
    registry: docker.io
    repository: prom/mysqld-exporter
    tag: v0.10.0
    pullPolicy: IfNotPresent
  resources: {}
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9104"
```

You can then override any of these settings in a YAML formatted file,
and then pass that file during installation.

```console
$ echo '{mariadbUser: user0, mariadbDatabase: user0db}' > config.yaml
$ helm install -f config.yaml stable/mariadb
```

The above will create a default MariaDB user with the name `user0`, and
grant this user access to a newly created `user0db` database, but will
accept all the rest of the defaults for that chart.

There are two ways to pass configuration data during install:

- `--values` (or `-f`): Specify a YAML file with overrides. This can be specified multiple times
  and the rightmost file will take precedence
- `--set` (and its variants `--set-string` and `--set-file`): Specify overrides on the command line.

If both are used, `--set` values are merged into `--values` with higher precedence.
Overrides specified with `--set` are persisted in a configmap. Values that have been
`--set` can be viewed for a given release with `helm get values <release-name>`. 
Values that have been `--set` can be cleared by running `helm upgrade` with `--reset-values`
specified.

#### The Format and Limitations of `--set`

The `--set` option takes zero or more name/value pairs. At its simplest, it is
used like this: `--set name=value`. The YAML equivalent of that is:

```yaml
name: value
```

Multiple values are separated by `,` characters. So `--set a=b,c=d` becomes:

```yaml
a: b
c: d
```

More complex expressions are supported. For example, `--set outer.inner=value` is
translated into this:
```yaml
outer:
  inner: value
```

Lists can be expressed by enclosing values in `{` and `}`. For example,
`--set name={a, b, c}` translates to:

```yaml
name:
  - a
  - b
  - c
```

As of Helm 2.5.0, it is possible to access list items using an array index syntax.
For example, `--set servers[0].port=80` becomes:

```yaml
servers:
  - port: 80
```

Multiple values can be set this way. The line `--set servers[0].port=80,servers[0].host=example` becomes:

```yaml
servers:
  - port: 80
    host: example
```

Sometimes you need to use special characters in your `--set` lines. You can use
a backslash to escape the characters; `--set name="value1\,value2"` will become:

```yaml
name: "value1,value2"
```

Similarly, you can escape dot sequences as well, which may come in handy when charts use the
`toYaml` function to parse annotations, labels and node selectors. The syntax for
`--set nodeSelector."kubernetes\.io/role"=master` becomes:

```yaml
nodeSelector:
  kubernetes.io/role: master
```

Deeply nested data structures can be difficult to express using `--set`. Chart
designers are encouraged to consider the `--set` usage when designing the format
of a `values.yaml` file.

Helm will cast certain values specified with `--set` to integers.
For example, `--set foo=true` results Helm to cast `true` into an int64 value.
In case you want a string, use a `--set`'s variant named `--set-string`. `--set-string foo=true` results in a string value of `"true"`.

`--set-file key=filepath` is another variant of `--set`.
It reads the file and use its content as a value.
An example use case of it is to inject a multi-line text into values without dealing with indentation in YAML.
Say you want to create a [brigade](https://github.com/Azure/brigade) project with certain value containing 5 lines JavaScript code, you might write a `values.yaml` like:

```yaml
defaultScript: |
  const { events, Job } = require("brigadier")
  function run(e, project) {
    console.log("hello default script")
  }
  events.on("run", run)
```

Being embedded in a YAML, this makes it harder for you to use IDE features and testing framework and so on that supports writing code.
Instead, you can use `--set-file defaultScript=brigade.js` with `brigade.js` containing:

```javascript
const { events, Job } = require("brigadier")
function run(e, project) {
  console.log("hello default script")
}
events.on("run", run)
```

### More Installation Methods

The `helm install` command can install from several sources:

- A chart repository (as we've seen above)
- A local chart archive (`helm install foo-0.1.1.tgz`)
- An unpacked chart directory (`helm install path/to/foo`)
- A full URL (`helm install https://example.com/charts/foo-1.2.3.tgz`)

## 'helm upgrade' and 'helm rollback': Upgrading a Release, and Recovering on Failure

When a new version of a chart is released, or when you want to change
the configuration of your release, you can use the `helm upgrade`
command.

An upgrade takes an existing release and upgrades it according to the
information you provide. Because Kubernetes charts can be large and
complex, Helm tries to perform the least invasive upgrade. It will only
update things that have changed since the last release.

```console
$ helm upgrade -f panda.yaml happy-panda stable/mariadb
Fetched stable/mariadb-0.3.0.tgz to /Users/mattbutcher/Code/Go/src/k8s.io/helm/mariadb-0.3.0.tgz
happy-panda has been upgraded. Happy Helming!
Last Deployed: Wed Sep 28 12:47:54 2016
Namespace: default
Status: DEPLOYED
...
```

In the above case, the `happy-panda` release is upgraded with the same
chart, but with a new YAML file:

```yaml
mariadbUser: user1
```

We can use `helm get values` to see whether that new setting took
effect.

```console
$ helm get values happy-panda
mariadbUser: user1
```

The `helm get` command is a useful tool for looking at a release in the
cluster. And as we can see above, it shows that our new values from
`panda.yaml` were deployed to the cluster.

Now, if something does not go as planned during a release, it is easy to
roll back to a previous release using `helm rollback [RELEASE] [REVISION]`.

```console
$ helm rollback happy-panda 1
```

The above rolls back our happy-panda to its very first release version.
A release version is an incremental revision. Every time an install,
upgrade, or rollback happens, the revision number is incremented by 1.
The first revision number is always 1. And we can use `helm history [RELEASE]`
to see revision numbers for a certain release.

## Helpful Options for Install/Upgrade/Rollback
There are several other helpful options you can specify for customizing the
behavior of Helm during an install/upgrade/rollback. Please note that this
is not a full list of cli flags. To see a description of all flags, just run
`helm <command> --help`.

- `--timeout`: A value in seconds to wait for Kubernetes commands to complete
  This defaults to 300 (5 minutes)
- `--wait`: Waits until all Pods are in a ready state, PVCs are bound, Deployments
  have minimum (`Desired` minus `maxUnavailable`) Pods in ready state and
  Services have an IP address (and Ingress if a `LoadBalancer`) before 
  marking the release as successful. It will wait for as long as the 
  `--timeout` value. If timeout is reached, the release will be marked as 
  `FAILED`. Note: In scenario where Deployment has `replicas` set to 1 and 
  `maxUnavailable` is not set to 0 as part of rolling update strategy, 
  `--wait` will return as ready as it has satisfied the minimum Pod in ready condition.
- `--no-hooks`: This skips running hooks for the command
- `--recreate-pods` (only available for `upgrade` and `rollback`): This flag
  will cause all pods to be recreated (with the exception of pods belonging to
  deployments)

## 'helm delete': Deleting a Release

When it is time to uninstall or delete a release from the cluster, use
the `helm delete` command:

```console
$ helm delete happy-panda
```

This will remove the release from the cluster. You can see all of your
currently deployed releases with the `helm list` command:

```console
$ helm list
NAME           	VERSION	UPDATED                        	STATUS         	CHART
inky-cat       	1      	Wed Sep 28 12:59:46 2016       	DEPLOYED       	alpine-0.1.0
```

From the output above, we can see that the `happy-panda` release was
deleted.

However, Helm always keeps records of what releases happened. Need to
see the deleted releases? `helm list --deleted` shows those, and `helm
list --all` shows all of the releases (deleted and currently deployed,
as well as releases that failed):

```console
⇒  helm list --all
NAME           	VERSION	UPDATED                        	STATUS         	CHART
happy-panda   	2      	Wed Sep 28 12:47:54 2016       	DELETED        	mariadb-0.3.0
inky-cat       	1      	Wed Sep 28 12:59:46 2016       	DEPLOYED       	alpine-0.1.0
kindred-angelf 	2      	Tue Sep 27 16:16:10 2016       	DELETED        	alpine-0.1.0
```

Because Helm keeps records of deleted releases, a release name cannot be
re-used. (If you _really_ need to re-use a release name, you can use the
`--replace` flag, but it will simply re-use the existing release and
replace its resources.)

Note that because releases are preserved in this way, you can rollback a
deleted resource, and have it re-activate.

## 'helm repo': Working with Repositories

So far, we've been installing charts only from the `stable` repository.
But you can configure `helm` to use other repositories. Helm provides
several repository tools under the `helm repo` command.

You can see which repositories are configured using `helm repo list`:

```console
$ helm repo list
NAME           	URL
stable         	https://kubernetes-charts.storage.googleapis.com
local          	http://localhost:8879/charts
mumoshu        	https://mumoshu.github.io/charts
```

And new repositories can be added with `helm repo add`:

```console
$ helm repo add dev https://example.com/dev-charts
```

Because chart repositories change frequently, at any point you can make
sure your Helm client is up to date by running `helm repo update`.

## Creating Your Own Charts

The [Chart Development Guide](charts.md) explains how to develop your own
charts. But you can get started quickly by using the `helm create`
command:

```console
$ helm create deis-workflow
Creating deis-workflow
```

Now there is a chart in `./deis-workflow`. You can edit it and create
your own templates.

As you edit your chart, you can validate that it is well-formatted by
running `helm lint`.

When it's time to package the chart up for distribution, you can run the
`helm package` command:

```console
$ helm package deis-workflow
deis-workflow-0.1.0.tgz
```

And that chart can now easily be installed by `helm install`:

```console
$ helm install ./deis-workflow-0.1.0.tgz
...
```

Charts that are archived can be loaded into chart repositories. See the
documentation for your chart repository server to learn how to upload.

Note: The `stable` repository is managed on the [Kubernetes Charts
GitHub repository](https://github.com/kubernetes/charts). That project
accepts chart source code, and (after audit) packages those for you.

## Tiller, Namespaces and RBAC
In some cases you may wish to scope Tiller or deploy multiple Tillers to a single cluster. Here are some best practices when operating in those circumstances.

1. Tiller can be [installed](install.md) into any namespace. By default, it is installed into kube-system. You can run multiple Tillers provided they each run in their own namespace.
2. Limiting Tiller to only be able to install into specific namespaces and/or resource types is controlled by Kubernetes [RBAC](https://kubernetes.io/docs/admin/authorization/rbac/) roles and rolebindings. You can add a service account to Tiller when configuring Helm via `helm init --service-account <NAME>`. You can find more information about that [here](rbac.md).
3. Release names are unique PER TILLER INSTANCE.
4. Charts should only contain resources that exist in a single namespace.
5. It is not recommended to have multiple Tillers configured to manage resources in the same namespace.

## Conclusion

This chapter has covered the basic usage patterns of the `helm` client,
including searching, installation, upgrading, and deleting. It has also
covered useful utility commands like `helm status`, `helm get`, and
`helm repo`.

For more information on these commands, take a look at Helm's built-in
help: `helm help`.

In the next chapter, we look at the process of developing charts.
