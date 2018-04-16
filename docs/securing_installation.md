# Securing your Helm Installation

Helm is a powerful and flexible package-management and operations tool for Kubernetes. Installing it using the default installation command -- `helm init` -- quickly and easily installs **Tiller**, the server-side component with which Helm corresponds. 

This default installation applies **_no security configurations_**, however. It's completely appropriate to use this type of installation when you are working against a cluster with no or very few security concerns, such as local development with Minikube or with a cluster that is well-secured in a private network with no data-sharing or no other users or teams. If this is the case, then the default installation is fine, but remember: With great power comes great responsibility. Always use due diligence when deciding to use the default installation.

## Who Needs Security Configurations?

For the following types of clusters we strongly recommend that you apply the proper security configurations to Helm and Tiller to ensure the safety of the cluster, the data in it, and the network to which it is connected.

- Clusters that are exposed to uncontrolled network environments: either untrusted network actors can access the cluster, or untrusted applications that can access the network environment.
- Clusters that are for many people to use -- _multitenant_ clusters -- as a shared environment
- Clusters that have access to or use high-value data or networks of any type

Often, environments like these are referred to as _production grade_ or _production quality_ because the damage done to any company by misuse of the cluster can be profound for either customers, the company itself, or both. Once the risk of damage becomes high enough, you need to ensure the integrity of your cluster no matter what the actual risk. 

To configure your installation properly for your environment, you must:

- Understand the security context of your cluster
- Choose the Best Practices you should apply to your helm installation

The following assumes you have a Kubernetes configuration file (a _kubeconfig_ file) or one was given to you to access a cluster. 

## Understanding the Security Context of your Cluster

`helm init` installs Tiller into the cluster in the `kube-system` namespace and without any RBAC rules applied. This is appropriate for local development and other private scenarios because it enables you to be productive immediately. It also enables you to continue running Helm with existing Kubernetes clusters that do not have role-based access control (RBAC) support until you can move your workloads to a more recent Kubernetes version.

There are four main areas to consider when securing a tiller installation:

1. Role-based access control, or RBAC
2. Tiller's gRPC endpoint and its usage by Helm
3. Tiller release information
4. Helm charts

### RBAC

Recent versions of Kubernetes employ a [role-based access control (or RBAC)](https://en.wikipedia.org/wiki/Role-based_access_control) system (as do modern operating systems) to help mitigate the damage that can done if credentials are misused or bugs exist. Even where an identity is hijacked, the identity has only so many permissions to a controlled space. This effectively adds a layer of security to limit the scope of any attack with that identity. 

Helm and Tiller are designed to install, remove, and modify logical applications that can contain many services interacting together. As a result, often its usefulness involves cluster-wide operations, which in a multitenant cluster means that great care must be taken with access to a cluster-wide Tiller installation to prevent improper activity. 

Specific users and teams -- developers, operators, system and network administrators -- will need their own portion of the cluster in which they can use Helm and Tiller without risking other portions of the cluster. This means using a Kubernetes cluster with RBAC enabled and Tiller configured to enforce them. For more information about using RBAC in Kubernetes, see [Using RBAC Authorization](rbac.md).

#### Tiller and User Permissions

Tiller in its current form does not provide a way to map user credentials to specific permissions within Kubernetes. When Tiller is running inside of the cluster, it operates with the permissions of its service account. If no service account name is supplied to Tiller, it runs with the default service account for that namespace. This means that all Tiller operations on that server are executed using the Tiller pod's credentials and permissions. 

To properly limit what Tiller itself can do, the standard Kubernetes RBAC mechanisms must be attached to Tiller, including Roles and RoleBindings that place explicit limits on what things a Tiller instance can install, and where. 

This situation may change in the future. While the community has several methods that might address this, at the moment performing actions using the rights of the client, instead of the rights of Tiller, is contingent upon the outcome of the Pod Identity Working Group, which has taken on the task of solving the problem in a general way. 


### The Tiller gRPC Endpoint and TLS

In the default installation the gRPC endpoint that Tiller offers is available inside the cluster (not external to the cluster) without authentication configuration applied. Without applying authentication, any process in the cluster can use the gRPC endpoint to perform operations inside the cluster. In a local or secured private cluster, this enables rapid usage and is normal. (When running outside the cluster, Helm authenticates through the Kubernetes API server to reach Tiller, leveraging existing Kubernetes authentication support.)

Shared and production clusters -- for the most part -- should use Helm 2.7.2 at a minimum and configure TLS for each Tiller gRPC endpoint to ensure that within the cluster usage of gRPC endpoints is only for the properly authenticated identity for that endpoint. Doing so enables any number of Tiller instances to be deployed in any number of namespaces and yet no unauthenticated usage of any gRPC endpoint is possible. Finally, usa Helm `init` with the `--tiller-tls-verify` option to install Tiller with TLS enabled and to verify remote certificates, and all other Helm commands should use the `--tls` option.

For more information about the proper steps to configure Tiller and use Helm properly with TLS configured, see [Using SSL between Helm and Tiller](tiller_ssl.md).

When Helm clients are connecting from outside of the cluster, the security between the Helm client and the API server is managed by Kubernetes itself. You may want to ensure that this link is secure. Note that if you are using the TLS configuration recommended above, not even the Kubernetes API server has access to the unencrypted messages between the client and Tiller.

### Tiller's Release Information

For historical reasons, Tiller stores its release information in ConfigMaps. We suggest changing the default to Secrets.

Secrets are the Kubernetes accepted mechanism for saving configuration data that is considered sensitive. While secrets don't themselves offer many protections, Kubernetes cluster management software often treats them differently than other objects. Thus, we suggest using secrets to store releases.

Enabling this feature currently requires setting the `--storage=secret` flag in the tiller-deploy deployment. This entails directly modifying the deployment or using `helm init --override=...`, as no helm init flag is currently available to do this for you. For more information, see [Using --override](install.md#using---override).

### Thinking about Charts

Because of the relative longevity of Helm, the Helm chart ecosystem evolved without the immediate concern for cluster-wide control, and especially in the developer space this makes complete sense. However, charts are a kind of package that not only installs containers you may or may not have validated yourself, but it may also install into more than one namespace. 

As with all shared software, in a controlled or shared environment you must validate all software you install yourself _before_ you install it. If you have secured Tiller with TLS and have installed it with permissions to only one or a subset of namespaces, some charts may fail to install -- but in these environments, that is exactly what you want. If you need to use the chart, you may have to work with the creator or modify it yourself in order to use it securely in a multitenant cluster with proper RBAC rules applied. The `helm template` command renders the chart locally and displays the output. 

Once vetted, you can use Helm's provenance tools to [ensure the provenance and integrity of charts](provenance.md) that you use.

### gRPC Tools and Secured Tiller Configurations

Many very useful tools use the gRPC interface directly, and having been built against the default installation -- which provides cluster-wide access -- may fail once security configurations have been applied. RBAC policies are controlled by you or by the cluster operator, and either can be adjusted for the tool, or the tool can be configured to work properly within the constraints of specific RBAC policies applied to Tiller. The same may need to be done if the gRPC endpoint is secured: the tools need their own secure TLS configuration in order to use a specific Tiller instance. The combination of RBAC policies and a secured gRPC endpoint configured in conjunction with gRPC tools enables you to control your cluster environment as you should.

## Best Practices for Securing Helm and Tiller

The following guidelines reiterate the Best Practices for securing Helm and Tiller and using them correctly. 

1. Create a cluster with RBAC enabled
2. Configure each Tiller gRPC endpoint to use a separate TLS certificate 
3. Release information should be a Kubernetes Secret 
4. Install one Tiller per user, team, or other organizational entity with the `--service-account` flag, Roles, and RoleBindings
5. Use the `--tiller-tls-verify` option with `helm init` and the `--tls` flag with other Helm commands to enforce verification

If these steps are followed, an example `helm init` command might look something like this: 
 
```bash
$ helm init \
--tiller-tls \
--tiller-tls-verify \
--tiller-tls-ca-cert=ca.pem \
--tiller-tls-cert=cert.pem \
--tiller-tls-key=key.pem \
--service-account=accountname  
```

This command will start Tiller with both strong authentication over gRPC, and a service account to which RBAC policies have been applied. 







