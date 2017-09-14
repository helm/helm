# Tiller and Service Accounts

In Kubernetes, granting a role to an application-specific service account is a best practice to ensure that your application is operating in the scope that you have specified. Read more about service account permissions [in the official Kubernetes docs](https://kubernetes.io/docs/admin/authorization/rbac/#service-account-permissions). Bitnami also has a fantastic guide for [configuring RBAC in your cluster](https://docs.bitnami.com/kubernetes/how-to/configure-rbac-in-your-kubernetes-cluster/) that takes you through RBAC basics.

You can add a service account to Tiller using the `--service-account <NAME>` flag while you're configuring helm. As a prerequisite, you'll have to create a role binding which specifies a [role](https://kubernetes.io/docs/admin/authorization/rbac/#role-and-clusterrole) and a [service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) name that have been set up in advance.

Once you have satisfied the pre-requisite and have a service account with the correct permissions, you'll run a command like this: `helm init --service-account <NAME>`

## Example: Service account with cluster-admin role

```console
$ kubectl create serviceaccount tiller --namespace kube-system
```

In `rbac-config.yaml`:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tiller
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: tiller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: tiller
    namespace: kube-system
```

_Note: The cluster-admin role is created by default in a Kubernetes cluster, so you don't have to define it explicitly._

```console
$ kubectl create -f rbac-config.yaml
$ helm init --service-account tiller
```

## Example: Service account restricted to a namespace
In the example above, we gave Tiller admin access to the entire cluster. You are not at all required to give Tiller cluster-admin access for it to work. Instead of specifying a ClusterRole or a ClusterRoleBinding, you can specify a Role and RoleBinding to limit Tiller's scope to a particular namespace.

```console
$ kubectl create namespace tiller-world
namespace "tiller-world" created
$ kubectl create serviceaccount tiller --namespace tiller-world
serviceaccount "tiller" created
```

Define a Role like in `role-tiller.yaml`:
```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  namespace: tiller-world
  name: tiller-manager
rules:
- apiGroups: ["", "extensions", "apps"]
  resources: ["deployments", "replicasets", "pods", "configmaps", "secrets", "namespaces"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"] # You can also use ["*"]
```

```console
$ kubectl create -f role-tiller.yaml
role "tiller-manager" created
```

In `rolebinding-tiller.yaml`,
```yaml
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-binding
  namespace: tiller-world
subjects:
- kind: ServiceAccount
  name: tiller
  namespace: tiller-world
roleRef:
  kind: Role
  name: tiller-manager
  apiGroup: rbac.authorization.k8s.io
```

```console
$ kubectl create -f rolebinding-tiller.yaml
rolebinding "tiller-binding" created
```

```console
$ helm init --service-account tiller --tiller-namespace tiller-world
$HELM_HOME has been configured at /Users/awesome-user/.helm.

Tiller (the helm server side component) has been installed into your Kubernetes Cluster.
Happy Helming!

$ helm install nginx --tiller-namespace tiller-world --namespace tiller-world
NAME:   wayfaring-yak
LAST DEPLOYED: Mon Aug  7 16:00:16 2017
NAMESPACE: tiller-world
STATUS: DEPLOYED

RESOURCES:
==> v1/Pod
NAME                  READY  STATUS             RESTARTS  AGE
wayfaring-yak-alpine  0/1    ContainerCreating  0         0s
```

