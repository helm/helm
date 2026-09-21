# Helm end-to-end tests

These tests exercise a real `helm` binary against real external systems: an OCI
registry, and optionally a Kubernetes cluster. They are guarded by the `e2e`
build tag, so they are never compiled or run by `make test`.

Unlike the unit tests, nothing here is faked. The registry is a real registry,
and the Kubernetes test installs into whatever cluster your current kubecontext
points at.

## Running

```console
$ export HELM_E2E_REGISTRY=ghcr.io/your-org
$ export HELM_E2E_USERNAME=your-user
$ export HELM_E2E_PASSWORD=your-token
$ make test-e2e
```

Any OCI registry works, not just GHCR. To run against a local registry:

```console
$ export HELM_E2E_REGISTRY=localhost:5000/charts
$ export HELM_E2E_PLAIN_HTTP=true
```

A default `registry:2` deployment serves anonymous access and accepts any
credential. Against such a registry there is no authentication to exercise, so
the invalid-credential test detects this and skips itself. Run against an
auth-enabled registry to cover that path.

## Configuration

| Variable               | Required | Description                                                                                    |
| ---------------------- | -------- | ---------------------------------------------------------------------------------------------- |
| `HELM_E2E_REGISTRY`    | yes      | Registry namespace to push to, without a scheme (e.g. `ghcr.io/your-org`).                       |
| `HELM_E2E_USERNAME`    | yes      | Username for the registry.                                                                       |
| `HELM_E2E_PASSWORD`    | yes      | Password or token for the registry. Passed to helm on stdin and never logged.                    |
| `HELM_E2E_BIN`         | no       | Path to a prebuilt helm binary. Defaults to building one from the working tree.                  |
| `HELM_E2E_PLAIN_HTTP`  | no       | Set to use plain HTTP, for registries without TLS.                                               |
| `HELM_E2E_KUBERNETES`  | no       | Set to run the install test against the current kubecontext. Off by default because it mutates.  |
| `HELM_E2E_NAMESPACE`   | no       | Namespace for the Kubernetes test. Defaults to a unique `helm-e2e-<run id>` namespace.           |

Because the `e2e` build tag is already an explicit opt-in, a missing required
variable fails the test rather than silently skipping it.

Each run pushes to repositories suffixed with a random run ID, so concurrent
runs and repeated runs in a shared namespace do not collide.

The Kubernetes test deletes the namespace it creates. If you supply
`HELM_E2E_NAMESPACE`, that namespace is left in place and only the releases are
uninstalled.
