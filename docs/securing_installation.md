# Securing your Helm Installation

Helm is a powerful and flexible package-management and operations tool for Kubernetes. Installing it using the default installation command -- `helm init` -- quickly and easily installs **Tiller**, the server-side component with which Helm corresponds. 

This default installation applies **_no security configurations_**, however. It's completely appropriate to use this type of installation when you are working against a cluster with no or very few security concerns, such as local development with Minikube or with a cluster that is well-secured in a private network with no data-sharing or no other users or teams. If this is the case, then the default installation is fine, but remember: With great power comes great responsibility. Always use due diligence when deciding to use the default installation.

## Who Needs Security Configurations?

For the following types of clusters we strongly recommend that you apply the proper security configurations to Helm and Tiller to ensure the safety of the cluster, the data in it, and the network to which it is connected.

- Clusters that are exposed to uncontrolled network environments
- Clusters that are for many people to use -- _multitenant_ clusters -- as a shared environment
- Clusters that have access to or use high-value data or networks of any type

Often, environments like these are referred to as _production grade_ or _production quality_ because the damage done to any company by misuse of the cluster can be profound for either customers, the company itself, or both. Once the risk of damage becomes high enough, you need to ensure the integrity of your cluster no matter what the actual risk. 

To configure your installation properly for your environment, you must:

- Understand the security context of your cluster
- Choose the Best Practices you should apply to your helm installation

The following assumes you have a Kubernetes configuration file (a _kubeconfig_ file) or one was given to you to access a cluster. 

## Understanding the Security Context of your Cluster

`helm init` installs Tiller into the cluster in the `default` namespace and without any RBAC rules applied. Again, this is entirely appropriate for local development and other private scenarios because it enables you to be productive immediately. It also enables you to continue running Helm with existing Kubernetes clusters that do not have role-based access control (RBAC) support until you can move your workloads to a more recent Kubernetes version.

There are four main areas to consider when securing a tiller installation:

1. Role-based access control, or RBAC
2. Tiller's gRPC endpoint and its usage by Helm
3. Tiller release information
4. Helm charts

### RBAC

Modern system design realizes that bugs exist and that humans can be fooled; Kubernetes is no different. This is why although such systems have user-based security enforcement mechanisms, additional enforcememt mechanisms exist to provide multiple layers of restrictions to enable the proper usage but protect against malicious exploitation of either a weakness in the code or access to a user's credentials. 

Recent versions of Kubernetes employ a role-based access control (or RBAC) system as do modern operating systems to help mitigate the damage that can done if credentials are misused or bugs exist.

### The Tiller gRPC Endpoint

### Tiller's Release Information

### Thinking about Charts

Charts can be vectors to install anything. 

## Best Practices for Securing Helm and Tiller


1. Create a cluster with RBAC enabled
2. To ensure trusted agent model is secure, you must Tiller gRPC with TLS 
3. Release information should be a Secret 
4. Install one Tiller per user, team, or other organizational entity
5. Use Helm with the `--tiller-tls-verify` flag to enforce verification