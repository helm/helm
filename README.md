# Helm

[![Circle CI](https://circleci.com/gh/kubernetes/helm.svg?style=svg)](https://circleci.com/gh/kubernetes/helm) [![Go Report Card](http://goreportcard.com/badge/kubernetes/helm)](http://goreportcard.com/report/kubernetes/helm)

Helm makes it easy to create, describe, update and
delete Kubernetes resources using declarative configuration. A configuration is
just a `YAML` file that configures Kubernetes resources or supplies parameters
to templates.

Helm Manager runs server side, in your Kubernetes cluster, so it can tell you what templates
you've instantiated there, what resources they created, and even how the resources
are organized. So, for example, you can ask questions like:

* What Redis instances are running in this cluster?
* What Redis master and slave services are part of this Redis instance?
* What pods are part of this Redis slave?

The official Helm repository of charts is available in the
[kubernetes/charts](https://github.com/kubernetes/charts) repository.

Please hang out with us in [the Slack chat room](https://kubernetes.slack.com/messages/helm/).

## Installing Helm

Note: if you're exploring or using the project, you'll probably want to pull
[the latest release](https://github.com/kubernetes/helm/releases/latest),
since there may be undiscovered or unresolved issues at HEAD.

From a Linux or Mac OS X client:

Ensure GOPATH is set.

Ensure you are authenticated against and are able to access a Kubernetes cluster.

```
$ git clone https://github.com/kubernetes/helm.git $GOPATH/src/github.com/kubernetes/helm
$ cd $GOPATH/src/github.com/kubernetes/helm
$ go get ./...
$ make build
$ cd $GOPATH
$ bin/helm server install
```

That's it. You can now use `kubectl` to see Helm running in your cluster like this:

```
$ kubectl get pod,rc,service --namespace=helm
NAME                    READY        STATUS        RESTARTS   AGE
expandybird-rc-e0whp    1/1          Running       0          35m
expandybird-rc-zdp8w    1/1          Running       0          35m
manager-rc-bl4i4        1/1          Running       0          35m
resourcifier-rc-21clg   1/1          Running       0          35m
resourcifier-rc-i2zhi   1/1          Running       0          35m
NAME                    CLUSTER-IP   EXTERNAL-IP   PORT(S)    AGE
expandybird-service     10.0.0.248   <none>        8081/TCP   35m
manager-service         10.0.0.49    <none>        8080/TCP   35m
resourcifier-service    10.0.0.184   <none>        8082/TCP   35m
NAME                    DESIRED      CURRENT       AGE
expandybird-rc          2            2             35m
manager-rc              1            1             35m
resourcifier-rc         2            2             35m
```

If you see expandybird, manager and resourcifier services, as well as expandybird, manager and resourcifier replication controllers with pods that are READY, then Helm is up and running!

## Using Helm

Run a Kubernetes proxy to allow the Helm client to connect to the remote cluster:

```
kubectl proxy --port=8001 &
```

Configure the HELM_HOST environment variable to let the local Helm client talk to the Helm manager service running in your remote Kubernetes cluster using the proxy.

```
export HELM_HOST=http://localhost:8001/api/v1/proxy/namespaces/helm/services/manager-service:manager
```

## Installing Charts

To quickly deploy a chart, you can use the Helm command line tool.

Currently here is the step by step guide.

First add a respository of Charts used for testing:

```
$ bin/helm repo add kubernetes-charts-testing gs://kubernetes-charts-testing
```

Then deploy a Chart from this repository. For example to start a Redis cluster:

```
$ bin/helm deploy --name test --properties "workers=2" gs://kubernetes-charts-testing/redis-2.0.0.tgz
```
The command above will create a helm "deployment" called `test` using the `redis-2.0.0.tgz` chart stored in the google storage bucket `kubernetes-charts-testing`.

`$ bin/helm deployment describe test` will allow you to see the status of the resources you just created using the redis-2.0.0.tgz chart. You can also use kubectl to see the the same resources. It'll look like this:

```
$ kubectl get pods,svc,rc
NAME                    READY        STATUS        RESTARTS   AGE
barfoo-barfoo           5/5          Running       0          45m
redis-master-rc-8wrqt   1/1          Running       0          41m
redis-slave-rc-6ptx6    1/1          Running       0          41m
redis-slave-rc-yc12q    1/1          Running       0          41m
NAME                    CLUSTER-IP   EXTERNAL-IP   PORT(S)    AGE
kubernetes              10.0.0.1     <none>        443/TCP    45m
redis-master            10.0.0.67    <none>        6379/TCP   41m
redis-slave             10.0.0.168   <none>        6379/TCP   41m
NAME                    DESIRED      CURRENT       AGE
redis-master-rc         1            1             41m
redis-slave-rc          2            2             41m
```

To connect to your Redis master with a local `redis-cli` just use `kubectl port-forward` in a similar manner to:

```
$ kubectl port-forward redis-master-rc-8wrqt 6379:639 &
$ redis-cli
127.0.0.1:6379> info
...
role:master
connected_slaves:2
slave0:ip=172.17.0.10,port=6379,state=online,offset=925,lag=0
slave1:ip=172.17.0.11,port=6379,state=online,offset=925,lag=1
```

Once you are done, you can delete your deployment with

```
$ bin/helm deployment list
test
$ bin/helm deployment rm test
````

## Uninstalling Helm from Kubernetes

You can uninstall Helm entirely using the following command:

```
$ bin/helm server uninstall
```

This command will remove everything in the Helm namespace being used.

## Design of Helm

There is a more detailed [design document](docs/design/design.md) available.

## Status of the Project

This project is still under active development, so you might run into issues. If
you do, please don't be shy about letting us know, or better yet, contribute a
fix or feature.

## Contributing
Your contributions are welcome.

We use the same [workflow](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/development.md#git-setup),
[License](LICENSE) and [Contributor License Agreement](CONTRIBUTING.md) as the main Kubernetes repository.

## Relationship to Google Cloud Platform's Deployment Manager and Deis's Helm
Kubernetes Helm represent a merge of Google's Deployment Manager (DM) and the original Helm from Deis.
Kubernetes Helm uses many of the same concepts and languages as
[Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/overview),
but creates resources in Kubernetes clusters, not in Google Cloud Platform projects. It also brings several concepts from the original Helm such as Charts.
