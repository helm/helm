# WordPress

[WordPress](https://wordpress.org/) is one of the most versatile open source content management systems on the market. A publishing platform for building blogs and websites.

## TL;DR;

```console
$ helm install stable/wordpress
```

## Introduction

This chart bootstraps a [WordPress](https://github.com/bitnami/bitnami-docker-wordpress) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

It also packages the [Bitnami MariaDB chart](https://github.com/kubernetes/charts/tree/master/stable/mariadb) which is required for bootstrapping a MariaDB deployment for the database requirements of the WordPress application.

## Prerequisites

- Kubernetes 1.4+ with Beta APIs enabled
- PV provisioner support in the underlying infrastructure

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm install --name my-release stable/wordpress
```

The command deploys WordPress on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the WordPress chart and their default values.

|            Parameter             |                Description                 |                         Default                         |
|----------------------------------|--------------------------------------------|---------------------------------------------------------|
| `image.registry`                 | WordPress image registry                   | `docker.io`                                             |
| `image.repository`               | WordPress image name                       | `bitnami/wordpress`                                     |
| `image.tag`                      | WordPress image tag                        | `{VERSION}`                                             |
| `image.pullPolicy`               | Image pull policy                          | `Always` if `imageTag` is `latest`, else `IfNotPresent` |
| `image.pullSecrets`              | Specify image pull secrets                 | `nil`                                                   |
| `wordpressUsername`              | User of the application                    | `user`                                                  |
| `wordpressPassword`              | Application password                       | _random 10 character long alphanumeric string_          |
| `wordpressEmail`                 | Admin email                                | `user@example.com`                                      |
| `wordpressFirstName`             | First name                                 | `FirstName`                                             |
| `wordpressLastName`              | Last name                                  | `LastName`                                              |
| `wordpressBlogName`              | Blog name                                  | `User's Blog!`                                          |
| `wordpressTablePrefix`           | Table prefix                               | `wp_`                                                   |
| `allowEmptyPassword`             | Allow DB blank passwords                   | `yes`                                                   |
| `smtpHost`                       | SMTP host                                  | `nil`                                                   |
| `smtpPort`                       | SMTP port                                  | `nil`                                                   |
| `smtpUser`                       | SMTP user                                  | `nil`                                                   |
| `smtpPassword`                   | SMTP password                              | `nil`                                                   |
| `smtpUsername`                   | User name for SMTP emails                  | `nil`                                                   |
| `smtpProtocol`                   | SMTP protocol [`tls`, `ssl`]               | `nil`                                                   |
| `replicaCount`                   | Number of WordPress Pods to run            | `1`                                                     |
| `mariadb.enabled`                | Deploy MariaDB container(s)                | `true`                                                  |
| `mariadb.rootUser.password`      | MariaDB admin password                     | `nil`                                                   |
| `mariadb.db.name`                | Database name to create                    | `bitnami_wordpress`                                     |
| `mariadb.db.user`                | Database user to create                    | `bn_wordpress`                                          |
| `mariadb.db.password`            | Password for the database                  | _random 10 character long alphanumeric string_          |
| `externalDatabase.host`          | Host of the external database              | `localhost`                                             |
| `externalDatabase.user`          | Existing username in the external db       | `bn_wordpress`                                          |
| `externalDatabase.password`      | Password for the above username            | `nil`                                                   |
| `externalDatabase.database`      | Name of the existing database              | `bitnami_wordpress`                                     |
| `externalDatabase.port`          | Database port number                       | `3306`                                                  |
| `serviceType`                    | Kubernetes Service type                    | `LoadBalancer`                                          |
| `serviceExternalTrafficPolicy`   | Enable client source IP preservation       | `Cluster`                                               |
| `nodePorts.http`                 | Kubernetes http node port                  | `""`                                                    |
| `nodePorts.https`                | Kubernetes https node port                 | `""`                                                    |
| `healthcheckHttps`               | Use https for liveliness and readiness     | `false`                                                 |
| `ingress.enabled`                | Enable ingress controller resource         | `false`                                                 |
| `ingress.hosts[0].name`          | Hostname to your WordPress installation    | `wordpress.local`                                       |
| `ingress.hosts[0].path`          | Path within the url structure              | `/`                                                     |
| `ingress.hosts[0].tls`           | Utilize TLS backend in ingress             | `false`                                                 |
| `ingress.hosts[0].tlsSecret`     | TLS Secret (certificates)                  | `wordpress.local-tls-secret`                            |
| `ingress.hosts[0].annotations`   | Annotations for this host's ingress record | `[]`                                                    |
| `ingress.secrets[0].name`        | TLS Secret Name                            | `nil`                                                   |
| `ingress.secrets[0].certificate` | TLS Secret Certificate                     | `nil`                                                   |
| `ingress.secrets[0].key`         | TLS Secret Key                             | `nil`                                                   |
| `persistence.enabled`            | Enable persistence using PVC               | `true`                                                  |
| `persistence.existingClaim`      | Enable persistence using an existing PVC   | `nil`                                                   |
| `persistence.storageClass`       | PVC Storage Class                          | `nil` (uses alpha storage class annotation)             |
| `persistence.accessMode`         | PVC Access Mode                            | `ReadWriteOnce`                                         |
| `persistence.size`               | PVC Storage Request                        | `10Gi`                                                  |
| `nodeSelector`                   | Node labels for pod assignment             | `{}`                                                    |
| `tolerations`                    | List of node taints to tolerate            | `[]`                                                    |
| `affinity`                       | Map of node/pod affinities                 | `{}`                                                    |

The above parameters map to the env variables defined in [bitnami/wordpress](http://github.com/bitnami/bitnami-docker-wordpress). For more information please refer to the [bitnami/wordpress](http://github.com/bitnami/bitnami-docker-wordpress) image documentation.

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install --name my-release \
  --set wordpressUsername=admin,wordpressPassword=password,mariadb.mariadbRootPassword=secretpassword \
    stable/wordpress
```

The above command sets the WordPress administrator account username and password to `admin` and `password` respectively. Additionally, it sets the MariaDB `root` user password to `secretpassword`.

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install --name my-release -f values.yaml stable/wordpress
```

> **Tip**: You can use the default [values.yaml](values.yaml)

## Production and horizontal scaling

The following repo contains the recommended production settings for wordpress capture in an alternative [values file](values-production.yaml). Please read carefully the comments in the values-production.yaml file to set up your environment appropriately.

To horizontally scale this chart, first download the [values-production.yaml](values-production.yaml) file to your local folder, then:

```console
$ helm install --name my-release -f ./values-production.yaml stable/wordpress
```

Note that [values-production.yaml](values-production.yaml) includes a replicaCount of 3, so there will be 3 WordPress pods. As a result, to use the /admin portal and to ensure you can scale wordpress you need to provide a ReadWriteMany PVC, if you don't have a provisioner for this type of storage, we recommend that you install the nfs provisioner and map it to a RWO volume.

```console
$ helm install stable/nfs-server-provisioner --set persistence.enabled=true,persistence.size=10Gi
$ helm install --name my-release -f values-production.yaml --set persistence.storageClass=nfs stable/wordpress
```

## Persistence

The [Bitnami WordPress](https://github.com/bitnami/bitnami-docker-wordpress) image stores the WordPress data and configurations at the `/bitnami` path of the container.

Persistent Volume Claims are used to keep the data across deployments. This is known to work in GCE, AWS, and minikube.
See the [Configuration](#configuration) section to configure the PVC or to disable persistence.

## Using an external database

Sometimes you may want to have Wordpress connect to an external database rather than installing one inside your cluster, e.g. to use a managed database service, or use run a single database server for all your applications. To do this, the chart allows you to specify credentials for an external database under the [`externalDatabase` parameter](#configuration). You should also disable the MariaDB installation with the `mariadb.enabled` option. For example:

```console
$ helm install stable/wordpress \
    --set mariadb.enabled=false,externalDatabase.host=myexternalhost,externalDatabase.user=myuser,externalDatabase.password=mypassword,externalDatabase.database=mydatabase,externalDatabase.port=3306
```

Note also if you disable MariaDB per above you MUST supply values for the `externalDatabase` connection.

## Ingress

This chart provides support for ingress resources. If you have an
ingress controller installed on your cluster, such as [nginx-ingress](https://kubeapps.com/charts/stable/nginx-ingress)
or [traefik](https://kubeapps.com/charts/stable/traefik) you can utilize
the ingress controller to serve your WordPress application.

To enable ingress integration, please set `ingress.enabled` to `true`

### Hosts

Most likely you will only want to have one hostname that maps to this
WordPress installation, however, it is possible to have more than one
host.  To facilitate this, the `ingress.hosts` object is an array.

For each item, please indicate a `name`, `tls`, `tlsSecret`, and any
`annotations` that you may want the ingress controller to know about.

Indicating TLS will cause WordPress to generate HTTPS URLs, and
WordPress will be connected to at port 443.  The actual secret that
`tlsSecret` references do not have to be generated by this chart.
However, please note that if TLS is enabled, the ingress record will not
work until this secret exists.

For annotations, please see [this document](https://github.com/kubernetes/ingress-nginx/blob/master/docs/annotations.md).
Not all annotations are supported by all ingress controllers, but this
document does a good job of indicating which annotation is supported by
many popular ingress controllers.

### TLS Secrets

This chart will facilitate the creation of TLS secrets for use with the
ingress controller, however, this is not required.  There are three
common use cases:

* helm generates/manages certificate secrets
* user generates/manages certificates separately
* an additional tool (like [kube-lego](https://kubeapps.com/charts/stable/kube-lego))
manages the secrets for the application

In the first two cases, one will need a certificate and a key.  We would
expect them to look like this:

* certificate files should look like (and there can be more than one
certificate if there is a certificate chain)

```
-----BEGIN CERTIFICATE-----
MIID6TCCAtGgAwIBAgIJAIaCwivkeB5EMA0GCSqGSIb3DQEBCwUAMFYxCzAJBgNV
...
jScrvkiBO65F46KioCL9h5tDvomdU1aqpI/CBzhvZn1c0ZTf87tGQR8NK7v7
-----END CERTIFICATE-----
```
* keys should look like:
```
-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAvLYcyu8f3skuRyUgeeNpeDvYBCDcgq+LsWap6zbX5f8oLqp4
...
wrj2wDbCDCFmfqnSJ+dKI3vFLlEz44sAV8jX/kd4Y6ZTQhlLbYc=
-----END RSA PRIVATE KEY-----
````

If you are going to use Helm to manage the certificates, please copy
these values into the `certificate` and `key` values for a given
`ingress.secrets` entry.

If you are going are going to manage TLS secrets outside of Helm, please
know that you can create a TLS secret by doing the following:

```
kubectl create secret tls wordpress.local-tls --key /path/to/key.key --cert /path/to/cert.crt
```

Please see [this example](https://github.com/kubernetes/contrib/tree/master/ingress/controllers/nginx/examples/tls)
for more information.
