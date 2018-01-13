# Export and import feature proposal

Export - ability to save currently installed releases into a single file (`helm export` command). Import - ability to restore (re-install)
previously exported releases (`helm import` command).

## Use cases

### Disaster recovery

A Cluster Administrator can run `helm export` command continuously or after releasing software. This command will save all Helm releases into a file.
It will include all data associated with the release including name, version, chart, values and release history. This file can be uploaded to S3 bucket where it
will be stored permanently. Later, in case of emergency, this file can be used to restore cluster state by running `helm import` command.

### Creating new environment

Typically, organizations run multiple environments. Example: DEV, STAGE, PROD. This can be achieved by either using a dedicated namespace in the single
Kubernetes cluster or by creating a dedicated Kubernetes cluster for every environment. In either case, `helm export` and `helm import` can be used for creating
new environments from scratch. `helm export` will save a snapshot of the environment and `helm import` will restore this snapshot in given Kubernetes cluster or namespace.

## Implementation

The following commands will be added to `helm` binary:

* `helm export`
* `helm import`

The following actions will be added to tiller's `ReleaseServer`:

* `func (s *ReleaseServer) Export(...)`
* `func (s *ReleaseServer) Import(...)`

### Export file format

```protobuf
syntax = "proto3";

package hapi.export;

message Metadata {
  google.protobuf.Timestamp created_at = 1; // Timestamp when export file was created
  string tiller_version = 2; // Version of tiller which was used
}

message Export {
  hapi.export.Metadata metadata = 1; // Export file metadata
  repeated hapi.release.Release releases = 2; // List of releases
}
```

The protobuf representation of `hapi.export.Export` will be additionally gzipped.

### helm export

`helm export` will send a new gRPC request to `tiller` asking to create a new export file. `tiller` will prepare and send this file back to the client.
All releases including the ones which were superseded by new versions will be saved.

Arguments:

* `--namespace` - Namespace to export. Defaults to the current kube config namespace.
* `--file` - Output file path. Defaults to "helm-releases-<timestamp>.gz".

### helm import

`helm import` will send the passed export file to `tiller` and tiller will install all releases from this file in the given namespace.

Arguments:

* `--file` (Mandatory) - Path to the export file
* `--namespace` - Namespace where tiller will re-install releases. Defaults to the current kube config namespace.
* `--no-hooks` - Skip hooks
* `--replace` - By default, helm will raise an error if any of the releases in the export file are already installed. This option will force tiller to replace them.

## Alternatives

* https://github.com/heptio/ark
