# Role-Based Access Control

This part of the Best Practices Guide discusses the creation and formatting of RBAC resources in chart manifests.

RBAC resources are:

- ServiceAccount (namespaced)
- Role (namespaced)
- ClusterRole 
- RoleBinding (namespaced)
- ClusterRoleBinding

## RBAC-related values

RBAC-related values in a chart should be namespaced under an `rbac` top-level key.  At a minimum this key should contain these sub-keys (explained below):

- `create`
- `serviceAccountName`

Other keys under `rbac` may also be listed and used as well.

## RBAC Resources Should be Created by Default

`rbac.create` should be a boolean value controlling whether RBAC resources are created.  The default should be `true`.  Users who wish to manage RBAC access controls themselves can set this value to `false` (in which case see below).

## Using RBAC Resources

`rbac.serviceAccountName` should set the name of the ServiceAccount to be used by access-controlled resources created by the chart.  If `rbac.create` is true, then a ServiceAccount with this name should be created.  If `rbac.create` is false, then it should not be created, but it should still be associated with the same resources so that manually-created RBAC resources created later that reference it will function correctly.  (Note that this effectively makes `rbac.serviceAccountName` a required value in these charts.)
