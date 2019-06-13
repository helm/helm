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
   - v2 and v3 history/state are independent of each other. v2 uses "ConfigMaps" under Tiller namespace and `TILLER`ownership. v3 uses "Secrets" in user namespace and `helm` ownership. There should be no conflicts.  Releases are incremental in both v2 and v3.
   - The only issue could be if Kubernetes cluster scoped resources (e.g. `clusterroles.rbac`) are defined in a chart. The v3 deployment would then fail even if unique in the namepsace as the resources would clash.
   - Make sure to not override the v2 client binary (`helm`). Rename or use separate directory for one of the releases.
   - v3 has configuration and when initialized will override the v2 configuration. To avoid this use a separate `HELM_HOME`, for example, `export HELM_HOME=$HOME/.helm3`.
 
2. Deploying a new chart:
   - Use v3 to deploy
 
3. Move an existing release from v2 to v3, choices as follows:
   - Lose the release history: Deploy as new using v3 and delete the existing release using v2.
   - Maintain the release history:
     - Retrieve v2 release states by getting the ConfigMaps from the `kube-system` namespace for `Tiller` owner

       ```console
       $ kubectl get configmap -n kube-system -l "OWNER=TILLER"
       NAME          DATA   AGE
       mychart.v1    1      27h
       easy-chrt.v1  1      26h
       ```

     - For each release and version (e.g. `mychart.v1`) you want to move::
       - Extract the data from the ConfigMap: `kubectl get configmap ${RELEASE_NAME} -n kube-system -o json > ${RELEASE_NAME}-cm.json`
       - Map it to a v3 Secret as follows:
         - Set owner to `helm`
         - Set namespace to namespace of release in v2. Check with command: `helm ls`

           ```console
           $ helm ls
           NAME    	REVISION	UPDATED                 	STATUS  	CHART         	APP VERSION	NAMESPACE
           mychart 	1       	Wed Jun 12 10:58:22 2019	DEPLOYED	mychart-0.1.0 	1.0        	default  
           easy-chrt	1       	Wed Jun 12 12:16:56 2019	DEPLOYED	new-chrt-0.1.0	1.0        	default  
           ```

         - Set 'kind' to `Secret`
         - Add type `helm.sh/release`
         - Update some keys from capital case to camel case and lower case

           ```
           # Make copy of the ConfigMap output
           cp ${RELEASE_NAME}-cm.json ${RELEASE_NAME}-secret.json

           # Update fields and values to correspond to v3 state secret object
           sed -i -e 's/ConfigMap/Secret/g' ./${RELEASE_NAME}-secret.json
           sed -i -e 's/MODIFIED_AT/modifiedAt/g' ./${RELEASE_NAME}-secret.json
           sed -i -e 's/NAME/name/g' ./${RELEASE_NAME}-secret.json
           sed -i -e 's/OWNER/owner/g' ./${RELEASE_NAME}-secret.json
           sed -i -e 's/STATUS/status/g' ./${RELEASE_NAME}-secret.json
           sed -i -e 's/VERSION/version/g' ./${RELEASE_NAME}-secret.json
           sed -i -e 's/configmaps/secrets/g' ./${RELEASE_NAME}-secret.json
           sed -i -e "s/kube-system/${NAMESPACE}/g" ./${RELEASE_NAME}-secret.json
           sed -i -e 's/TILLER/helm/g' ./${RELEASE_NAME}-secret.json
           STATUS=`jq '.metadata.labels.status' ${RELEASE_NAME}-secret.json | tr '[:upper:]' '[:lower:]'`
           jq ".metadata.labels.status=${STATUS}" ${RELEASE_NAME}-secret.json > ${RELEASE_NAME}-secret.tmp && mv ${RELEASE_NAME}-secret.tmp ${RELEASE_NAME}-secret.json
           ```

           *** Note: The release data in the ConfigMap is a base-64 encoded, gzipped archive of the entire release record. TODO: This is currently failing to be loaded by v3. ***

       - Create the Secret resource in the namespace of the release

         ```
         # Deploy the ${RELEASE_NAME} secret into the ${NAMESPACE} namespace
         kubens ${NAMESPACE}
         kubectl create -f ${RELEASE_NAME}-secret.json
         ```

       - Check the release now exists in v3 (`helm ls`) and has state stored as a Secret (`kubectl get secret --all-namespaces -l "owner=helm"`):

         ```console
         $ helm ls
         NAME     	NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART          
         mychart  	default  	1       	2019-06-12 10:43:19.949644311 +0100 IST	deployed	mychart-0.1.0  
         easy-chrt	default  	1       	2019-06-12 10:09:20.903353326 +0100 IST	deployed	easy-chrt-0.1.0
         demo     	default  	1       	2019-06-12 14:31:52.264875915 +0100 IST	deployed	demo-0.1.0    

         $ kubectl get secret --all-namespaces -l "owner=helm"
         NAMESPACE   NAME           TYPE              DATA   AGE
         default     demo.v1        helm.sh/release   1      23h
         default     easy-chrt.v1   helm.sh/release   1      28h
         default     mychart.v1     helm.sh/release   1      27h
         ```

       - Delete the release ConfigMap: `kubectl delete configmap ${RELEASE_NAME} -n kube-system`
 
4. Move Helm releaes and it's Kubernetes resources from it's default v2 namespace (only current release version applicable with namespaced scoped resources):
   - Get all resources from the current release: `helm get <release>`
   - For each resource:
     - Create the resource in the new namespace: `kubectl get <resource_type> <resource_name> -o json --namespace <ns_old> | jq '.items[].metadata.namespace = "ns_new"' | kubectl create-f  -`
     - Delete the resource in the old namespace: `kubectl delete <resource_type> --namespace <ns_old>`
   - Update the release Secret resource to new namespace: `kubectl edit secret <release_name> -n <<ns_old>>`

