# Quickstart Guide

This guide covers how you can quickly get started using Helm.

## Prerequisites

The following prerequisites are required for a successful and properly secured use of Helm.

1. A Kubernetes cluster
2. Deciding what security configurations to apply to your installation, if any
3. Installing and configuring Helm and Tiller, the cluster-side service.


### Install Kubernetes or have access to a cluster
- You must have Kubernetes installed. For the latest release of Helm, we recommend the latest stable release of Kubernetes, which in most cases is the second-latest minor release.
- You should also have a local configured copy of `kubectl`.

NOTE: Kubernetes versions prior to 1.6 have limited or no support for role-based access controls (RBAC).

Helm will figure out where to install Tiller by reading your Kubernetes
configuration file (usually `$HOME/.kube/config`). This is the same file
that `kubectl` uses.

To find out which cluster Tiller would install to, you can run
`kubectl config current-context` or `kubectl cluster-info`.

```console
$ kubectl config current-context
my-cluster
```

### Understand your Security Context

As with all powerful tools, ensure you are installing it correctly for your scenario.

If you're using Helm on a cluster that you completely control, like minikube or a cluster on a private network in which sharing is not a concern, the default installation -- which applies no security configuration -- is fine, and it's definitely the easiest. To install Helm without additional security steps, [install Helm](#Install-Helm) and then [initialize Helm](#initialize-helm-and-install-tiller).

However, if your cluster is exposed to a larger network or if you share your cluster with others -- production clusters fall into this category -- you must take extra steps to secure your installation to prevent careless or malicious actors from damaging the cluster or its data. To apply configurations that secure Helm for use in production environments and other multi-tenant scenarios, see [Securing a Helm installation](securing_installation.md)

If your cluster has Role-Based Access Control (RBAC) enabled, you may want
to [configure a service account and rules](rbac.md) before proceeding.

## Install Helm

Download a binary release of the Helm client. You can use tools like
`homebrew`, or look at [the official releases page](https://github.com/helm/helm/releases).

For more details, or for other options, see [the installation
guide](install.md).

## Initialize Helm and Install Tiller

Once you have Helm ready, you can initialize the local CLI and also
install Tiller into your Kubernetes cluster in one step:

```console
$ helm init
```

This will install Tiller into the Kubernetes cluster you saw with
`kubectl config current-context`.

**TIP:** Want to install into a different cluster? Use the
`--kube-context` flag.

**TIP:** When you want to upgrade Tiller, just run `helm init --upgrade`.

By default, when Tiller is installed, it does not have authentication enabled.
To learn more about configuring strong TLS authentication for Tiller, consult
[the Tiller TLS guide](tiller_ssl.md).

## Install an Example Chart

To install a chart, you can run the `helm install` command. Helm has
several ways to find and install a chart, but the easiest is to use one
of the official `stable` charts.

```console
$ helm repo update              # Make sure we get the latest list of charts
$ helm install stable/mysql
NAME:   wintering-rodent
LAST DEPLOYED: Thu Oct 18 14:21:18 2018
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
==> v1/Secret
NAME                    AGE
wintering-rodent-mysql  0s

==> v1/ConfigMap
wintering-rodent-mysql-test  0s

==> v1/PersistentVolumeClaim
wintering-rodent-mysql  0s

==> v1/Service
wintering-rodent-mysql  0s

==> v1beta1/Deployment
wintering-rodent-mysql  0s

==> v1/Pod(related)

NAME                                    READY  STATUS   RESTARTS  AGE
wintering-rodent-mysql-6986fd6fb-988x7  0/1    Pending  0         0s


NOTES:
MySQL can be accessed via port 3306 on the following DNS name from within your cluster:
wintering-rodent-mysql.default.svc.cluster.local

To get your root password run:

    MYSQL_ROOT_PASSWORD=$(kubectl get secret --namespace default wintering-rodent-mysql -o jsonpath="{.data.mysql-root-password}" | base64 --decode; echo)

To connect to your database:

1. Run an Ubuntu pod that you can use as a client:

    kubectl run -i --tty ubuntu --image=ubuntu:16.04 --restart=Never -- bash -il

2. Install the mysql client:

    $ apt-get update && apt-get install mysql-client -y

3. Connect using the mysql cli, then provide your password:
    $ mysql -h wintering-rodent-mysql -p

To connect to your database directly from outside the K8s cluster:
    MYSQL_HOST=127.0.0.1
    MYSQL_PORT=3306

    # Execute the following command to route the connection:
    kubectl port-forward svc/wintering-rodent-mysql 3306

    mysql -h ${MYSQL_HOST} -P${MYSQL_PORT} -u root -p${MYSQL_ROOT_PASSWORD}

```

In the example above, the `stable/mysql` chart was released, and the name of
our new release is `wintering-rodent`. You get a simple idea of the
features of this MySQL chart by running `helm inspect stable/mysql`.

Whenever you install a chart, a new release is created. So one chart can
be installed multiple times into the same cluster. And each can be
independently managed and upgraded.

The `helm install` command is a very powerful command with many
capabilities. To learn more about it, check out the [Using Helm
Guide](using_helm.md)

## Learn About Releases

It's easy to see what has been released using Helm:

```console
$ helm ls
NAME            	REVISION	UPDATED                 	STATUS  	CHART       	APP VERSION	NAMESPACE
wintering-rodent	1       	Thu Oct 18 15:06:58 2018	DEPLOYED	mysql-0.10.1	5.7.14     	default
```

The `helm list` function will show you a list of all deployed releases.

## Uninstall a Release

To uninstall a release, use the `helm delete` command:

```console
$ helm delete wintering-rodent
release "wintering-rodent" deleted
```

This will uninstall `wintering-rodent` from Kubernetes, but you will
still be able to request information about that release:

```console
$ helm status wintering-rodent
LAST DEPLOYED: Thu Oct 18 14:21:18 2018
NAMESPACE: default
STATUS: DELETED

NOTES:
MySQL can be accessed via port 3306 on the following DNS name from within your cluster:
wintering-rodent-mysql.default.svc.cluster.local

To get your root password run:

    MYSQL_ROOT_PASSWORD=$(kubectl get secret --namespace default wintering-rodent-mysql -o jsonpath="{.data.mysql-root-password}" | base64 --decode; echo)

To connect to your database:

1. Run an Ubuntu pod that you can use as a client:

    kubectl run -i --tty ubuntu --image=ubuntu:16.04 --restart=Never -- bash -il

2. Install the mysql client:

    $ apt-get update && apt-get install mysql-client -y

3. Connect using the mysql cli, then provide your password:
    $ mysql -h wintering-rodent-mysql -p

To connect to your database directly from outside the K8s cluster:
    MYSQL_HOST=127.0.0.1
    MYSQL_PORT=3306

    # Execute the following command to route the connection:
    kubectl port-forward svc/wintering-rodent-mysql 3306

    mysql -h ${MYSQL_HOST} -P${MYSQL_PORT} -u root -p${MYSQL_ROOT_PASSWORD}
```

Because Helm tracks your releases even after you've deleted them, you
can audit a cluster's history, and even undelete a release (with `helm
rollback`).

## Reading the Help Text

To learn more about the available Helm commands, use `helm help` or type
a command followed by the `-h` flag:

```console
$ helm get -h
```
