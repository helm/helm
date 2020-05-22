# Deprecated Kubernetes APIs

Kubernetes is an API-driven system and the API evolves over time to reflect
the evolving understanding of the problem space. This is common practice
across systems and their APIs. An important part of evolving APIs is a good
deprecation policy and process to inform users of how changes to APIs are
implemented. In other words, consumers of your API need to know in advance and
in what release an API will be removed or changed. This removes the element of
surprise and breaking changes to consumers.

The [Kubernetes deprecation policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/)
documents how Kubernetes handles the changes to its API versions. The policy
for deprecation states the timeframe that API versions will be supported following
a deprecation announcement. It is therefore important to be aware of deprecation
announcements and know when API versions will be removed, to help minimize the
effect.

This is an example of an announcement [for the removal of deprecated API versions in Kubernetes 1.16](https://kubernetes.io/blog/2019/07/18/api-deprecations-in-1-16/)
and was advertised a few months prior to the release. These API versions would
have been announced for deprecation prior to this again. This shows that there
is a good policy in place which informs consumers of API version support. 

Helm templates specify a [Kubernetes API group](https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-groups)
when defining a Kubernetes object, similar to a Kubernetes manifest file. It is
specified in the `apiVersion` field of the template and it identifies the API
version of the Kubernetes object. This means that Helm users and chart maintainers
need to be aware when Kubernetes API versions have been deprecated and in what
Kubernetes version they will be removed.

## Chart Maintainers

You should audit your charts checking for Kubernetes API versions that are
deprecated or are removed in a Kubernetes version. The API versions found as
due to be or that are now out of support, should be updated to the supported
version and a new version of the chart released. The API version is defined by
the `kind` and `apiVersion` fields. For example, here is a removed `Deployment`
object API version in Kubernetes 1.16:

```yaml
apiVersion: apps/v1beta1
kind: Deployment
```

## Helm Users

You should audit the charts that you use (similar to [chart maintainers](#chart-maintainers))
and identify any charts where API versions are deprecated or removed in a
Kubernetes version. For the charts identified, you need to check for the latest
version of the chart (which has supported API versions) or update the chart
yourself.

Additionally, you also need to audit any charts deployed (i.e. Helm releases)
checking again for any deprecated or removed API versions. This can be done by
getting details of a release using the `helm get manifest` command.

The means for updating a Helm release to supported APIs depends on your findings
as follows:

1. If you find deprecated API versions only then:
  - Perform a `helm upgrade` with a version of the chart with supported Kubernetes
  API versions
  - Add a description in the upgrade, something along the lines to not perform a
  rollback to a Helm version prior to this current version
2.  If you find any API version(s) that is/are removed in a Kubernetes version
then:
  - If you are running a Kubernetes version where the API version(s) are still
  available (for example, you are on Kubernetes 1.15 and found you use APIs that
  will be removed in Kubernetes 1.16):
    - Follow the step 1 procedure
  - Otherwise (for example, you are already running a Kubernetes version where
  some API versions reported by `helm get manifest` are no longer available):
    - You need to edit the release manifest that is stored in the cluster to
    update the API versions to supported APIs. See
    [Updating API Versions of a Release Manifest](#updating-api-versions-of-a-release-manifest)
    for more details

> Note: In all cases of updating a Helm release with supported APIs, you should
never rollback the release to a version prior to the release version with the
supported APIs.

> Recommendation: The best practice is to upgrade releases using deprecated API
versions to supported API versions, prior to upgrading to a kubernetes cluster
that removes those API versions. 

If you don't update a release as suggested previously, you will have an error
similar to the following when trying to upgrade a release in a Kubernetes version
where its API version(s) is/are removed:

```
Error: UPGRADE FAILED: current release manifest contains removed kubernetes api(s)
for this kubernetes version and it is therefore unable to build the kubernetes
objects for performing the diff. error from kubernetes: unable to recognize "":
no matches for kind "Deployment" in version "apps/v1beta1"
```

Helm fails in this scenario because it attempts to create a diff patch between
the current deployed release (which contains the Kubernetes APIs that are removed
in this Kubernetes version) against the chart you are passing with the
updated/supported API versions. The underlying reason for failure is that when
Kubernetes removes an API version, the Kubernetes Go client library can no longer
parse the deprecated objects and Helm therefore fails when calling the library.
Helm unfortunately is unable to recover from this situation and is no longer able
to manage such a release.
See [Updating API Versions of a Release Manifest](#updating-api-versions-of-a-release-manifest)
for more details on how to recover from this scenario.

## Updating API Versions of a Release Manifest

The manifest is a property of the Helm release object which is stored in the data
field of a ConfigMap (default) or Secret in the cluster. The data field contains
a gzipped [protobuf object](developers#grpc-and-protobuf) which is base 64
encoded (there is an additional base 64 encoding for a Secret). There is
a Secret/ConfigMap per release version/revision in the namespace of the release.

You can use the Helm [mapkubeapis](https://github.com/hickeyma/helm-mapkubeapis)
plugin to perform the update of a release to supported APIs. Check out the
readme for more details.

Alternatively, you can follow these manual steps to perform an update of the API
versions of a release manifest. Depending on your configuration you will follow
the steps for the ConfigMap or Secret backend.

- Prerequisites:
  - HELM_PROTOBUF_SCHEMA: [Helm protobuf schema](https://github.com/helm/helm/tree/dev-v2/_proto)
  - PROTOBUF_SCHEMA: [Protobuf base schema](https://github.com/protocolbuffers/protobuf/tree/master/src) 
- Get the name of the ConfigMap or Secret associated with the latest deployed release:
  - ConfigMap backend: `kubectl get configmap -l OWNER=TILLER,STATUS=DEPLOYED,NAME=<release_name> --namespace <tiller_namespace> | awk '{print $1}' | grep -v NAME`
  - Secrets backend: `kubectl get secret -l OWNER=TILLER,STATUS=DEPLOYED,NAME=<release_name> --namespace <tiller_namespace> | awk '{print $1}' | grep -v NAME`
- Get latest deployed release details:
  - ConfigMap backend: `kubectl get configmap <release_configmap_name> -n <tiller_namespace> -o yaml > release.yaml`
  - Secrets backend: `kubectl get secret <release_secret_name> -n <tiller_namespace> -o yaml > release.yaml`
- Backup the release in case you need to restore if something goes wrong:
  - `cp release.yaml release.bak`
  - In case of emergency, restore: `kubectl apply -f release.bak -n <tiller_namespace>`
- Decode the release object: 
  - ConfigMap backend: `cat release.yaml | grep -oP '(?<=release: ).*' | base64 -d | gzip -d | protoc --proto_path ${HELM_PROTOBUF_SCHEMA} --proto_path ${PROTOBUF_SCHEMA} --decode hapi.release.Release ${HELM_PROTOBUF_SCHEMA}/hapi/**/* > release.data.decoded`
  - Secrets backend:`cat release.yaml | grep -oP '(?<=release: ).*' | base64 -d | base64 -d | gzip -d | protoc --proto_path ${HELM_PROTOBUF_SCHEMA} --proto_path ${PROTOBUF_SCHEMA} --decode hapi.release.Release ${HELM_PROTOBUF_SCHEMA}/hapi/**/* > release.data.decoded`
- Change API versions of the manifests. Can use any tool (e.g. editor) to make
the changes. This is in the `manifest` field of your decoded release
object (`release.data.decoded`)
- Encode the release object:
  - ConfigMap backend: `cat release.data.decoded | protoc --proto_path ${HELM_PROTOBUF_SCHEMA} --proto_path ${PROTOBUF_SCHEMA} --encode hapi.release.Release ${HELM_PROTOBUF_SCHEMA}/hapi/**/* | gzip | base64 --wrap 0`
  - Secrets backend: `cat release.data.decoded | protoc --proto_path ${HELM_PROTOBUF_SCHEMA} --proto_path ${PROTOBUF_SCHEMA} --encode hapi.release.Release ${HELM_PROTOBUF_SCHEMA}/hapi/**/* | gzip | base64 | base64 --wrap 0`
- Replace `data.release` property value in the deployed release file (`release.yaml`)
with the new encoded release object
- Apply file to namespace: `kubectl apply -f release.yaml -n <tiller_namespace>`
- Perform a `helm upgrade` with a version of the chart with supported Kubernetes
API versions
- Add a description in the upgrade, something along the lines to not perform a
rollback to a Helm version prior to this current version

> Note: Ensure to use the `protobuf schema` for the deployed Tiller version, otherwise the decoding might fail
