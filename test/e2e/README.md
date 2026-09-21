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
| `HELM_E2E_REPO`        | no       | Repository path under the registry. Defaults to `helm-e2e`.                                      |
| `HELM_E2E_ISOLATE`     | no       | Append a per-run suffix to the repository path. Off by default; see below.                       |
| `HELM_E2E_BIN`         | no       | Path to a prebuilt helm binary. Defaults to building one from the working tree.                  |
| `HELM_E2E_PLAIN_HTTP`  | no       | Set to use plain HTTP, for registries without TLS.                                               |
| `HELM_E2E_KUBERNETES`  | no       | Set to run the install test against the current kubecontext. Off by default because it mutates.  |
| `HELM_E2E_NAMESPACE`   | no       | Namespace for the Kubernetes test. Defaults to a unique `helm-e2e-<run id>` namespace.           |

Because the `e2e` build tag is already an explicit opt-in, a missing required
variable fails the test rather than silently skipping it.

## Repository layout

Helm appends the chart name to the push target, so a run touches one repository
per fixture:

```
<HELM_E2E_REGISTRY>/<HELM_E2E_REPO>/test
<HELM_E2E_REGISTRY>/<HELM_E2E_REPO>/compressedchart
<HELM_E2E_REGISTRY>/<HELM_E2E_REPO>/compressedchart-with-hyphens
<HELM_E2E_REGISTRY>/<HELM_E2E_REPO>/unicode-chart
```

That set is stable by default. The fixtures are immutable, so re-running
overwrites each tag with byte-identical content: concurrent runs do not
interfere, a shared registry does not accumulate repositories over time, and
registries that require a repository to exist before a push (ECR) can have
these created ahead of time.

Set `HELM_E2E_ISOLATE` to append a per-run suffix instead, which is useful when
several people share one registry namespace and you want a run to stand alone.
Note that stable paths stay safe only while every test pushes identical content
under a given tag; a test that pushes mutated content under a fixed tag would
need isolation.

## Running against several registries

The suite has no registry-specific behavior — everything comes from the
environment — so covering a new registry is a matter of pointing the same tests
at it. This is how `.github/workflows/e2e-registries.yml` exercises GHCR, Quay,
and ECR on a schedule, one matrix leg each, to catch changes that break a
particular registry implementation rather than OCI in general.

Two registry differences are worth knowing about:

- A default `registry:2` serves anonymous access, so the invalid-credential
  test skips itself there, as described above.
- ECR does not create repositories on push and issues a short-lived token
  rather than accepting a static password. The workflow creates the four
  repositories listed above and mints a token before running the tests.

The Kubernetes test deletes the namespace it creates. If you supply
`HELM_E2E_NAMESPACE`, that namespace is left in place and only the releases are
uninstalled.
