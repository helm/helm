Migration from v2 to v3 Helm 
===

## Table of Contents

- [Helm 3 Changes](#helm-3-changes)
- [Migration Use Cases](#migration-use-cases)

## Helm 3 Changes

The changes are as follows:

1. Removal of Tiller: 
   - Replaces client/server with client/library architecture
   - Security is now on per user basis (delegated to Kubernetes RBAC)
   - State stored as in-cluster Secrets
   - State persisted on a namespace basis and no longer global (release names now unique in namespace??) 
2. Chart repository updated:
   - Support added for upload of a chart to a repo (`helm push`)
   - Authentication supported for repo login
   - Authorization (OAuth2) supported which enables chart upload/install access rights
   - Support added for OCI registries (`helm chart <sub_command>`) 
3. Additional changes:
   - Add schema to values which enables value validation at runtime (install/upgrade etc.)
   - Helm install/set-up is simplified:
     - Helm client (helm binary) only (no Tiller)
     - No Helm initialization and no Tiller installation
     - Run-as-is paradigm
   - Commands removed/replaced/added:
       -  chart: command consists of multiple subcommands to interact with charts and registries. Subcommands as follows:
           - export: export a chart to directory
           - list: list all saved charts
           - pull: pull a chart from remote
           - push: push a chart to remote
           - remove: remove a chart
           - save: save a chart directory  
       -  delete --> uninstall : removes all release history by default (previous;y needed `--purge`)
       -  fetch --> pull
       -  init (removed?)
       -  install: requires release name or `--generate-name` argument
       -  inspect --> show
       -  reset (removed)
       -  serve (removed)
       -  upgrade: Added argument `maxHistory` which limits the maximum number of revisions saved per release (0 for no limit)
   - Improvements to error responses
   - Improvements on command argument validation and argument naming
   - Dynamically linked chart deopendencies moved to `Chart.yaml` (`requirements.yaml` removed and  requirements --> dependencies)
   - Library charts (helper charts) can now be added as dynamically linked chart dependencies

## Migration Use Cases

The migration use cases are as follows:

1. Running Helm v2 and v3 concurrently on the same cluster:
   - v2 and v3 history/state are independent of each other. v2 uses "Configmaps" under Tiller namespace and ownership. v3 uses "Secrets" in user namespace and Helm ownership. There should be no conflicts.
   - The only issue could be if Kubernetes resources are not named with unique capability like with rease in its name and you depoy the chart again for v3. This would happen for v2 anyway when not an upgrade. Can be avoided by naming resources uniqyely.
   - Make sure to not override the v2 binary (`helm`)
   - TBC: Will v3 have configuration and if so will it be the same directory as v2?
 
2. Deploying a new chart:
   - Use v3 to deploy
 
3. Upgrading an existing release:
   - Deploy as new on v3 and delete the existing release on v2
 
4. Migrate state of existing v2 releases:
   - TBD: Detail the steps involved in migration v2 envoironment to v3. This should describe the steps manually and then any tools provided for those manual steps.

