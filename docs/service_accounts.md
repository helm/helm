# Tiller and Service Accounts

In Kubernetes, granting a role to an application-specific service account is a best practice to ensure that your application is operating in the scope that you have specified. Read more about service account permissions in Kubernetes [here](https://kubernetes.io/docs/admin/authorization/rbac/#service-account-permissions).

You can add a service account to Tiller using the `--service-account <NAME>` flag while you're configuring helm. As a prerequisite, you'll have to create a role binding which specifies a [role](https://kubernetes.io/docs/admin/authorization/rbac/#role-and-clusterrole) and a [service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) name that have been set up in advance.

Once you have satisfied the pre-requisite and have a service account with the correct permissions, you'll run a command like this: `helm init --service-account <NAME>`

## Example

In `rbac-config.yaml`:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: helm
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: helm
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: helm
    namespace: kube-system
```


```console
$ kubectl create -f rbac-config.yaml
$ helm init --service-account helm
```

_Note: You do not have to specify a ClusterRole or a ClusterRoleBinding. You can specify a Role and RoleBinding instead to limit Tiller's scope to a particular namespace_
