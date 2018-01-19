# Role-Based Access Control

This part of the Best Practices Guide discusses the creation and formatting of RBAC resources in chart manifests.

RBAC resources are:

- ServiceAccount (namespaced)
- Role (namespaced)
- ClusterRole 
- RoleBinding (namespaced)
- ClusterRoleBinding

## YAML Configuration

RBAC and ServiceAccount configuration should happen under separate keys. They are separate things. Splitting these two concepts out in the YAML disambiguates them and make this clearer.

```yaml
rbac:
  # Specifies whether RBAC resources should be created
  create: true

serviceAccount:
  # Specifies whether a ServiceAccount should be created
  create: true
  # The name of the ServiceAccount to use.
  # If not set and create is true, a name is generated using the fullname template
  name:
```

This structure can be extended for more complex charts that require multiple ServiceAccounts.

```yaml
serviceAccounts:
  client:
    create: true
    name:
  server: 
    create: true
    name:
```

## RBAC Resources Should be Created by Default

`rbac.create` should be a boolean value controlling whether RBAC resources are created.  The default should be `true`.  Users who wish to manage RBAC access controls themselves can set this value to `false` (in which case see below).

## Using RBAC Resources

`serviceAccount.name` should set to the name of the ServiceAccount to be used by access-controlled resources created by the chart.  If `serviceAccount.create` is true, then a ServiceAccount with this name should be created.  If the name is not set, then a name is generated using the `fullname` template, If `serviceAccount.create` is false, then it should not be created, but it should still be associated with the same resources so that manually-created RBAC resources created later that reference it will function correctly.  If `serviceAccount.create` is false and the name is not specified, then the default ServiceAccount is used.

The following helper template should be used for the ServiceAccount.

```yaml
{{/*
Create the name of the service account to use
*/}}
{{- define "mychart.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "mychart.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
```
