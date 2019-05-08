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
   - State now stored as in-cluster Secrets
   - State persisted on a namespace basis and no longer global (release names now unique only in a namespace) 
2. Chart repository updated:
   - Support added for upload of a chart to a repo (`helm push`)
   - Support added for OCI registries (`helm chart <sub_command>`) :
     - Authentication supported for repo login/logout (`helm registry <sub_command>`)
     - Authorization (OAuth2) supported which enables chart upload/install access rights
3. Additional changes:
   - Add schema to values which enables value validation at runtime (install/upgrade etc.)
   - Helm install/set-up is simplified:
     - Helm client (helm) only (no tiller)
     - No Helm initialization and no Tiller installation
     - Run-as-is paradigm
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
       - init (removed?)
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
   - Dynamically linked chart deopendencies moved to `Chart.yaml` (`requirements.yaml` removed and  requirements --> dependencies)
   - Library charts (helper charts) can now be added as dynamically linked chart dependencies
   - Charts have a `type` metadata field to define the chart to be of an `application` or `library` chart. It is application by
     default which means it is renderable and installable.
   - Go import path is changed from `k8s.io/helm` to `helm.sh/helm` 

## Migration Use Cases

The migration use cases are as follows:

1. Running Helm v2 and v3 concurrently on the same cluster:
   - v2 and v3 history/state are independent of each other. v2 uses "ConfigMaps" under Tiller namespace and `TILLER`ownership. v3 uses "Secrets" in user namespace and `helm` ownership. There should be no conflicts.
   - The only issue could be if Kubernetes resources are not named with unique capability like with release in its name and you depoy the chart again for v3. This would happen for v2 anyway when not an upgrade. Can be avoided by naming resources uniquely.
   - Make sure to not override the v2 client binary (`helm`). Rename or separate directory.
   - v3 has no configuration and therefore doesn't need to be initialized. Run as is.
 
2. Deploying a new chart:
   - Use v3 to deploy
 
3. Upgrading an existing release from v2 to v3:
   - The choices are as follows:
     - Lose the release history: Deploy as new using v3 and delete the existing release using v2
     - Maintain the release history: TBD: Detail the steps involved in migration of v2 history to v3. This should describe the steps manually and then any tools provided for those manual steps. Are the steps involved:
       - Retrieve ConfigMaps for Tiller owner
       - Find ConfigMaps for a release
       - For each release:
         - Extract the data from the ConfigMap and map it to a Secret:
           - Ownere is `helm` and namespace is set 
           - Releases are incremental in both v2 and v3:

             ```console
             $ kubectl get configmap -n kube-system -l "OWNER=TILLER"
             NAME                      DATA   AGE
             cautious-hummingbird.v1   1      5d2h
             chrt-5586.v1              1      6d21h
             chrt-5586.v2              1      6d21h

             $ kubectl get secret -n default -l "owner=helm"
             NAME                 TYPE              DATA   AGE
             foo-chrt.v1          helm.sh/release   1      7d20h
             fuzzy-bear.v1        helm.sh/release   1      7d
             moo-chrt.v1          helm.sh/release   1      6d4h
             moo-chrt.v2          helm.sh/release   1      5s
             tst-lib-chart-1.v1   helm.sh/release   1      7d
             tst-lib-chart.v1     helm.sh/release   1      7d4h
             ```

           - The release data in the config map is a base-64 encoded, gzipped archive of the entire release record.
         - Create secret under a user provided or a default user if not provided. 
       - Delete the release ConfigMaps
 
4. Move release Kubernetes resources to user namespace
   - Get all resources from the release state, update the namespace and then update the resource. Suggestion here: https://gist.github.com/simonswine/6bf3b665e4117f42b550c3ea12dd171a

