# Role-based Access Control

In Kubernetes, granting a role to an application-specific service account is a best practice to ensure that your application is operating in the scope that you have specified. Read more about service account permissions [in the official Kubernetes docs](https://kubernetes.io/docs/admin/authorization/rbac/#service-account-permissions).

Bitnami also has a fantastic guide for [configuring RBAC in your cluster](https://docs.bitnami.com/kubernetes/how-to/configure-rbac-in-your-kubernetes-cluster/) that takes you through RBAC basics.

This guide is for users who want to restrict tiller's capabilities to install resources to certain namespaces, or to grant a helm client running access to a tiller instance.

## Tiller and Role-based Access Control

You can add a service account to Tiller using the `--service-account <NAME>` flag while you're configuring helm. As a prerequisite, you'll have to create a role binding which specifies a [role](https://kubernetes.io/docs/admin/authorization/rbac/#role-and-clusterrole) and a [service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) name that have been set up in advance.

Once you have satisfied the pre-requisite and have a service account with the correct permissions, you'll run a command like this: `helm init --service-account <NAME>`

### Example: Service account with cluster-admin role

```console
$ kubectl create serviceaccount tiller --namespace kube-system
serviceaccount "tiller" created
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
serviceaccount "tiller" created
clusterrolebinding "tiller" created
$ helm init --service-account tiller
```

### Example: Deploy tiller in a namespace, restricted to deploying resources only in that namespace

In the example above, we gave Tiller admin access to the entire cluster. You are not at all required to give Tiller cluster-admin access for it to work. Instead of specifying a ClusterRole or a ClusterRoleBinding, you can specify a Role and RoleBinding to limit Tiller's scope to a particular namespace.

```console
$ kubectl create namespace tiller-world
namespace "tiller-world" created
$ kubectl create serviceaccount tiller --namespace tiller-world
serviceaccount "tiller" created
```

Define a Role that allows tiller to manage all resources in `tiller-world` like in `role-tiller.yaml`:

```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-manager
  namespace: tiller-world
rules:
- apiGroups: ["", "extensions", "apps"]
  resources: ["*"]
  verbs: ["*"]
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

Afterwards you can run `helm init` to install tiller in the `tiller-world` namespace.

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

### Example: Deploy tiller in a namespace, restricted to deploying resources in another namespace

In the example above, we gave Tiller admin access to the namespace it was deployed inside. Now, let's limit Tiller's scope to deploy resources in a different namespace!

For example, let's install tiller in the namespace `myorg-system` and allow tiller to deploy resources in the namespace `myorg-users`.

```console
$ kubectl create namespace myorg-system
namespace "myorg-system" created
$ kubectl create serviceaccount tiller --namespace myorg-system
serviceaccount "tiller" created
```

Define a Role that allows tiller to manage all resources in `myorg-users` like in `role-tiller.yaml`:

```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-manager
  namespace: myorg-users
rules:
- apiGroups: ["", "extensions", "apps"]
  resources: ["*"]
  verbs: ["*"]
```

```console
$ kubectl create -f role-tiller.yaml
role "tiller-manager" created
```

Bind the service account to that role. In `rolebinding-tiller.yaml`,

```yaml
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-binding
  namespace: myorg-users
subjects:
- kind: ServiceAccount
  name: tiller
  namespace: myorg-system
roleRef:
  kind: Role
  name: tiller-manager
  apiGroup: rbac.authorization.k8s.io
```

```console
$ kubectl create -f rolebinding-tiller.yaml
rolebinding "tiller-binding" created
```

We'll also need to grant tiller access to read configmaps in myorg-system so it can store release information. In `role-tiller-myorg-system.yaml`:

```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  namespace: myorg-system
  name: tiller-manager
rules:
- apiGroups: ["", "extensions", "apps"]
  resources: ["configmaps"]
  verbs: ["*"]
```

```console
$ kubectl create -f role-tiller-myorg-system.yaml
role "tiller-manager" created
```

And the respective role binding. In `rolebinding-tiller-myorg-system.yaml`:

```yaml
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-binding
  namespace: myorg-system
subjects:
- kind: ServiceAccount
  name: tiller
  namespace: myorg-system
roleRef:
  kind: Role
  name: tiller-manager
  apiGroup: rbac.authorization.k8s.io
```

```console
$ kubectl create -f rolebinding-tiller-myorg-system.yaml
rolebinding "tiller-binding" created
```

## Helm and Role-based Access Control

When running a helm client in a pod, in order for the helm client to talk to a tiller instance, it will need certain privileges to be granted. Specifically, the helm client will need to be able to create pods, forward ports and be able to list pods in the namespace where tiller is running (so it can find tiller).

### Example: Deploy Helm in a namespace, talking to Tiller in another namespace

In this example, we will assume tiller is running in a namespace called `tiller-world` and that the helm client is running in a namespace called `helm-world`. By default, tiller is running in the `kube-system` namespace.

In `helm-user.yaml`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: helm
  namespace: helm-world
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: tiller-user
  namespace: tiller-world
rules:
- apiGroups:
  - ""
  resources:
  - pods/portforward
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - list
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: tiller-user-binding
  namespace: tiller-world
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: tiller-user
subjects:
- kind: ServiceAccount
  name: helm
  namespace: helm-world
```

```console
$ kubectl create -f helm-user.yaml
serviceaccount "helm" created
role "tiller-user" created
rolebinding "tiller-user-binding" created
```
