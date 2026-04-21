# oras-go v3 Functional Test Plan

Tests for [PR #32065](https://github.com/helm/helm/pull/32065) — oras-go v3 migration.

## Environment

- Kind cluster (`kind` context)
- Local registry at `localhost:5001` (Docker container, plain HTTP)
- Helm binary built from branch: `./bin/helm`

## Setup

```bash
# Set kubectl context
kubectl config use-context kind

# Start local OCI registry
docker run -d -p 5001:5000 --name oci-test-registry registry:2

# Build helm from branch
cd ~/code/helm && make build
alias helm=~/code/helm/bin/helm

# Package a test chart (use an existing one in the repo)
helm package scripts/charts/chart/ -d /tmp/helm-test-charts/
# or create a minimal one:
helm create /tmp/testchart
helm package /tmp/testchart -d /tmp/helm-test-charts/
```

---

## Step 1 — Login / Logout (plain HTTP registry)

Tests basic credential flow through new oras-go v3 auth path.

```bash
helm registry login localhost:5001 --username testuser --password testpass --plain-http
```
**Expected**: `Login Succeeded`; entry appears in `~/.config/helm/registry/config.json`.

```bash
helm registry logout localhost:5001
```
**Expected**: `Removed login credentials`; entry removed from config file.

---

## Step 2 — Push / Pull (smoke test)

Tests that reference parsing (`properties.NewReference`) and push/pull work end-to-end.

```bash
helm registry login localhost:5001 --plain-http
helm push /tmp/helm-test-charts/testchart-0.1.0.tgz oci://localhost:5001/charts
```
**Expected**: Output shows digest; no errors.

```bash
helm pull oci://localhost:5001/charts/testchart --version 0.1.0 --plain-http --destination /tmp/
```
**Expected**: `.tgz` downloaded to `/tmp/`.

---

## Step 3 — Show / Template from OCI

Tests the config-driven path through `show` and `template` commands.

```bash
helm show chart oci://localhost:5001/charts/testchart --version 0.1.0 --plain-http
helm show values oci://localhost:5001/charts/testchart --version 0.1.0 --plain-http
helm template myrelease oci://localhost:5001/charts/testchart --version 0.1.0 --plain-http
```
**Expected**: Chart metadata, values, and rendered YAML printed without errors.

---

## Step 4 — Install / Upgrade / Rollback / Uninstall from OCI (kind cluster)

End-to-end lifecycle test using the kind cluster.

```bash
kubectl config use-context kind
helm install myapp oci://localhost:5001/charts/testchart --version 0.1.0 --plain-http
helm list
```
**Expected**: Release `myapp` in `deployed` state.

```bash
# Push a v0.2.0 (bump version in Chart.yaml, repackage, push)
helm upgrade myapp oci://localhost:5001/charts/testchart --version 0.2.0 --plain-http
helm history myapp
```
**Expected**: Two revisions; latest is `deployed`.

```bash
helm rollback myapp 1
helm history myapp
```
**Expected**: Three revisions; latest is `deployed` (rolled back to v0.1.0).

```bash
helm uninstall myapp
```
**Expected**: `release "myapp" uninstalled`.

---

## Step 5 — Location Rewrite (Go integration test)

**Note**: `ConfigOptions`/`registries.conf` is not surfaced as a CLI flag, so this
must be tested via a Go integration test rather than the CLI.

Test verifies `Login()` stores credentials under the canonical host, not the alias.

```go
// pkg/registry/integration_test.go (new file, build tag: integration)
// 1. Write temp registries.conf:
//    [[registry]]
//    prefix = "alias.local"
//    location = "localhost:5001"
//    insecure = true
// 2. Create client with ClientOptConfigOptions pointing to that file
// 3. client.Login("alias.local", ...)
// 4. Assert credential stored under "localhost:5001" in the credentials store
// 5. Assert no entry under "alias.local"
// 6. client.Logout("alias.local")
// 7. Assert "localhost:5001" entry removed
```

Run with:
```bash
go test -v -tags integration ./pkg/registry/... -run TestLogin_LocationRewrite
```

---

## Step 6 — Policy Enforcement (Go integration test)

Tests `ClientOptPolicyEvaluator` deny-all blocks operations.

**Note**: `policy.json` is not surfaced as a CLI flag; must be tested in Go.

```go
// 1. Create a deny-all PolicyEvaluator
// 2. Build client with ClientOptPolicyEvaluator(denyAll)
// 3. Attempt pull → expect policy-rejection error
// 4. Build client with allow policy
// 5. Attempt pull → expect success
```

Run with:
```bash
go test -v ./pkg/registry/... -run TestPolicy
```

---

## Step 7 — registryAuthorizer Legacy Path (unit test)

Tests that `ClientOptRegistryAuthorizer` is now used when the legacy path is active.

A unit test using a `mockRemoteClient` injected via `ClientOptRegistryAuthorizer`
and `ClientOptHTTPClient` (to trigger `customHTTPClient=true`) should confirm the
mock client's `Do` is called during a registry operation.

```bash
go test -v ./pkg/registry/... -run TestRegistryAuthorizer
```

---

## Pass Criteria

| Test | Pass condition |
|------|---------------|
| Step 1 Login/Logout | No error; config.json updated |
| Step 2 Push/Pull | Digest printed; tgz downloaded |
| Step 3 Show/Template | Output rendered without error |
| Step 4 Install/Upgrade/Rollback/Uninstall | All lifecycle transitions succeed in kind |
| Step 5 Location Rewrite | Credential under canonical host only |
| Step 6 Policy Enforcement | Deny-all blocks; allow passes |
| Step 7 registryAuthorizer | Mock client's Do() invoked |
