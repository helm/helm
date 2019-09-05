Migration from v2 to v3 Helm 
===

## Table of Contents

- [Helm 3 Changes](#helm-3-changes)
- [Migration Use Cases](#migration-use-cases)

## Helm 3 Changes

The changes are as follows:

1. Removal of Tiller: 
   - Replaces client/server with client/library architecture (`Helm` binary only)
   - Security is now on per user basis (delegated to Kubernetes user cluster security)
   - State now stored as in-cluster Secrets and release object metadata has changed
   - State persisted on a namespace basis and no longer global (release names now unique only in a namespace) 
2. Chart repository updated:
   - Support added for upload of a chart to a repo (`helm push`)
   - helm search now supports both local repository searches and making search queries against Helm Hub
   - Experimental support added for OCI registries (`helm chart <sub_command>`) :
     - Authentication supported for repo login/logout (`helm registry <sub_command>`)
     - Authorization (OAuth2) supported which enables chart upload/install access rights
3. Chart apiVersion bumped to "v2" for following specification changes:
   - Dynamically linked chart deopendencies moved to `Chart.yaml` (`requirements.yaml` removed and  requirements --> dependencies)
   - Library charts (helper/common charts) can now be added as dynamically linked chart dependencies
   - Charts have a `type` metadata field to define the chart to be of an `application` or `library` chart. It is application by
     default which means it is renderable and installable
4. XDG directory specification added:
   - Helm home removed and replaced with XDG directory specification for storing configuration files which
   - No longer need to initialize Helm
   - `helm init` and `helm home` removed
5. Additional changes:
   - Add schema to values which enables value validation at runtime (install/upgrade etc.)
   - Helm install/set-up is simplified:
     - Helm client (helm) only (no tiller)
     - Run-as-is paradigm
   - `stable` repository not set-up by default
   - `crd-install` hook removed and replaced with `crd` directory in chart where all CRDs defined in it will be installed before any rendering of the chart
   - `test-failure` target removed. Use `test-success` instead
   - Commands removed/replaced/added:
       - chart: command consists of multiple subcommands to interact with charts and registries. Subcommands as follows:
         - export: export a chart to directory
         - list: list all saved charts
         - pull: pull a chart from remote
         - push: push a chart to remote
         - remove: remove a chart
         - save: save a chart directory  
       - delete --> uninstall : removes all release history by default (previous;y needed `--purge`)
       - fetch --> pull
       - home (removed)
       - init (removed)
       - install: requires release name or `--generate-name` argument
       - inspect --> show
       - registry: login to or logout from a registry
         - login: login to a registry
         - logout: logout from a registry
       - reset (removed)
       - serve (removed)
       - upgrade: Added argument `maxHistory` which limits the maximum number of revisions saved per release (0 for no limit)
   - Improvements to error responses
   - Improvements on command argument validation and argument naming
   - Go import path is changed from `k8s.io/helm` to `helm.sh/helm` 
   - Helm SDK updated. It is no longer compatible with Helm 2 library

## Migration Use Cases

The migration use cases are as follows:

1. Running Helm v2 and v3 concurrently on the same cluster:
   - v2 and v3 release (history) storage are independent of each other. The changes includes the Kubernetes resource for storage and the release object metadata contained in the resource. Releases will also be on a per user namespace instead of using the the Tiller namespace (for example, v2 default Tiller namespace kube-system).v2 uses "ConfigMaps" or "Secrets" under Tiller namespace and `TILLER`ownership. v3 uses "Secrets" in user namespace and `helm` ownership. There should be no conflicts.  Releases are incremental in both v2 and v3.
   - The only issue could be if Kubernetes cluster scoped resources (e.g. `clusterroles.rbac`) are defined in a chart. The v3 deployment would then fail even if unique in the namepsace as the resources would clash.
   - Make sure to not override the v2 client binary (`helm`). Rename or use separate directory for one of the releases.
   - v3 configuration no longer uses `HELM_HOME` and uses XDG directory specification instead. It is also created on the fly as need be. It is therefore independent of v2 configuration.
   - Note: This use case is only recommended if you intend to phase out Helm v2 gradually and do not require v3 to manage any releases deployed by v2. All new releases being deployed are performed by v3 and existing v2 deployed releases are updated/removed by v2 only.
 
2. Migrating Helm v2 to Helm v3:
   - Migrating Helm v2 configuration (`helm home`) to Helm v3:
     - - Proposal is a standalone migration tool that migrates from Helm v2 to Helm v3: https://github.com/helm/helm/issues/6154
   - Migrating Helm v2 releases in-place to v3:
     - Proposal is a standalone migration tool that migrates from Helm v2 to Helm v3: https://github.com/helm/helm/issues/6154
