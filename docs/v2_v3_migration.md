Helm v2 to v3 Migration 
===

## Table of Contents

- [Overview of Helm 3 Changes](#helm-3-changes)
- [Migration Use Cases](#migration-use-cases)

## Overview of Helm 3 Changes

The full list of changes from Helm 2 to 3 are documented in the [FAQ section](https://v3.helm.sh/docs/faq/#changes-since-helm-2).
The following is a summary of some of those changes that a user should be aware of before and during migration:

1. Removal of Tiller: 
   - Replaces client/server with client/library architecture (`helm` binary only)
   - Security is now on per user basis (delegated to Kubernetes user cluster security)
   - Releases are now stored as in-cluster secrets and the release object metadata has changed
   - Releases are persisted on a release namespace basis and not in the Tiller namepsace anymore
2. Chart repository updated:
   - `helm search` now supports both local repository searches and making search queries against Helm Hub
3. Chart apiVersion bumped to "v2" for following specification changes:
   - Dynamically linked chart dependencies moved to `Chart.yaml` (`requirements.yaml` removed and  requirements --> dependencies)
   - Library charts (helper/common charts) can now be added as dynamically linked chart dependencies
   - Charts have a `type` metadata field to define the chart to be of an `application` or `library` chart. It is application by
     default which means it is renderable and installable
   - Helm 2 charts (apiVersion=v1) are still installable
4. XDG directory specification added:
   - Helm home removed and replaced with XDG directory specification for storing configuration files
   - No longer need to initialize Helm
   - `helm init` and `helm home` removed
5. Additional changes:
   - Helm install/set-up is simplified:
     - Helm client (helm) only (no tiller)
     - Run-as-is paradigm
   - `local` or `stable` repositories are not set-up by default
   - `crd-install` hook removed and replaced with `crd` directory in chart where all CRDs defined in it will be installed before any rendering of the chart
   - `test-failure` target removed. Use `test-success` instead
   - Commands removed/replaced/added:
       - delete --> uninstall : removes all release history by default (previously needed `--purge`)
       - fetch --> pull
       - home (removed)
       - init (removed)
       - install: requires release name or `--generate-name` argument
       - inspect --> show
       - reset (removed)
       - serve (removed)
       - upgrade: Added argument `maxHistory` which limits the maximum number of revisions saved per release (0 for no limit)
   - Helm 3 Go library has undergone a lot of changes and it incompatible with the Helm 2 library
   - Release binaries are now hosted on `get.helm.sh`

## Migration Use Cases

The migration use cases are as follows:

1. Helm v2 and v3 managing the same cluster:
   - This use case is only recommended if you intend to phase out Helm v2 gradually and do not require v3 to manage any releases deployed by v2. All new releases being deployed should be performed by v3 and existing v2 deployed releases are updated/removed by v2 only
   - Helm v2 and v3 can quite happily manage the same cluster. The Helm versions can be installed on the same or separate systems
   - If installing Helm v3 on the same system, you need to to perform an additional step to ensure that both client versions can co-exist until ready to remove Helm v2 client. Rename or put the Helm v3 binary in a different folder to avoid conflict
   - Otherwise there are no conflicts between both versions because of the following distinctions: 
     - v2 and v3 release (history) storage are independent of each other. The changes includes the Kubernetes resource for storage and the release object metadata contained in the resource. Releases will also be on a per user namespace instead of using the the Tiller namespace (for example, v2 default Tiller namespace kube-system). v2 uses "ConfigMaps" or "Secrets" under the Tiller namespace and `TILLER`ownership. v3 uses "Secrets" in the user namespace and `helm` ownership. Releases are incremental in both v2 and v3
     - The only issue could be if Kubernetes cluster scoped resources (e.g. `clusterroles.rbac`) are defined in a chart. The v3 deployment would then fail even if unique in the namespace as the resources would clash
     - v3 configuration no longer uses `HELM_HOME` and uses XDG directory specification instead. It is also created on the fly as need be. It is therefore independent of v2 configuration. This is applicable only when both versions are installed on the same system
 
2. Migrating Helm v2 to Helm v3:
   - This use case applies when you want Helm v3 to manage existing Helm v2 releases
   - It should be noted that a Helm client: 
     - can manage 1 to many Kubernetes clusters
     - can connect to 1 to many Tiller instances for  a cluster 
   - This means that you have to cognisant of this when migrating as releases are deployed into clusters by Tiller and its namespace. You have to therefore be aware of migrating for each cluster and each Tiller instance that is managed by the Helm v2 client instance
   - The recommended data migration path is as follows:
     1. Backup v2 data
     2. Migrate Helm v2 configuration
     3. Migrate Helm v2 releases
     4. When happy that Helm v3 is managing all Helm v2 data (for all clusters and Tiller instances of the Helm v2 client instance) as expected, then clean up Helm v2 data
   - The migration process is automated by the Helm [2to3](https://github.com/helm/helm-2to3) plugin
