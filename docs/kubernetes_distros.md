# Kubernetes Distribution Guide

This document captures information about using Helm in specific Kubernetes
environments.

We are trying to add more details to this document. Please contribute via Pull
Requests if you can.

## MiniKube

Helm is tested and known to work with [minikube](https://github.com/kubernetes/minikube).
It requires no additional configuration.

## `scripts/local-cluster` and Hyperkube

Hyperkube configured via `scripts/local-cluster.sh` is known to work. For raw
Hyperkube you may need to do some manual configuration.

## GKE

Google's GKE hosted Kubernetes platform is known to work with Helm, and requires
no additional configuration.

## Ubuntu with 'kubeadm'

Kubernetes bootstrapped with `kubeadm` is known to work on the following Linux
distributions:

- Ubuntu 16.04
- Fedora release 25

Some versions of Helm (v2.0.0-beta2) require you to `export KUBECONFIG=/etc/kubernetes/admin.conf`
or create a `~/.kube/config`.

## Container Linux by CoreOS

Helm requires that kubelet have access to a copy of the `socat` program to proxy connections to the Tiller API. On Container Linux the Kubelet runs inside of a [hyperkube](https://github.com/kubernetes/kubernetes/tree/master/cluster/images/hyperkube) container image that has socat. So, even though Container Linux doesn't ship `socat` the container filesystem running kubelet does have socat. To learn more read the [Kubelet Wrapper](https://coreos.com/kubernetes/docs/latest/kubelet-wrapper.html) docs.

## Openshift

Helm works straightforward on OpenShift Online, OpenShift Dedicated, OpenShift Container Platform (version >= 3.6) or OpenShift Origin (version >= 3.6). To learn more read [this blog](https://blog.openshift.com/getting-started-helm-openshift/) post.

## Platform9

Helm Client and Helm Server (Tiller) are pre-installed with [Platform9 Managed Kubernetes](https://platform9.com/managed-kubernetes/?utm_source=helm_distro_notes). Platform9 provides access to all official Helm charts through the App Catalog UI and native Kubernetes CLI. Additional repositories can be manually added. Further details are availble in this [Platform9 App Catalog article](https://platform9.com/support/deploying-kubernetes-apps-platform9-managed-kubernetes/?utm_source=helm_distro_notes).