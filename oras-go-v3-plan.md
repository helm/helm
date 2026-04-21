# Helm oras-go v3 Migration Plan

## Overview

claude --resume 09e86345-a27b-4b2e-ba9a-c1829f3917e8

Migrate `helm.sh/helm/v4/pkg/registry` from `oras.land/oras-go/v2` to
`github.com/oras-project/oras-go/v3` and leverage the new v3 configuration
features to support the full container ecosystem config stack.

Local dev replace directive: `replace github.com/oras-project/oras-go/v3 => ./oras-go`

References:
- `./oras-go/MIGRATION_GUIDE.md` — v2→v3 breaking changes
- `./oras-go/docs/SCENARIOS.md` — v3 usage patterns

---

## Part 1: Mechanical Migration (v2 → v3) ✅ DONE

Commit: `build(deps): migrate oras-go v2 to v3`

### Changes made

| Area | v2 | v3 |
|------|----|----|
| Module path | `oras.land/oras-go/v2` | `github.com/oras-project/oras-go/v3` |
| Credential type | `auth.Credential` | `credentials.Credential` |
| Auth client field | `authorizer.Credential` | `authorizer.CredentialFunc` |
| Credential from store | `credentials.Credential(store)` | `remote.NewCredentialFunc(store)` |
| Static credential | `auth.StaticCredential(host, ...)` | `credentials.StaticCredentialFunc(host, ...)` |
| Login | Ping-probe + `ForceAttemptOAuth2` | `remote.Login(ctx, store, reg, cred)` |
| Logout | `credentials.Logout(...)` | `remote.Logout(ctx, store, host)` |
| Repository TLS/client | `repo.PlainHTTP`, `repo.Client` | `repo.Registry.PlainHTTP`, `repo.Registry.Client` |
| Store options | `DetectDefaultNativeStore: true` | removed (field renamed/inverted) |
| Config loader init | n/a | `_ "…/registry/remote/config"` blank import |

---

## Part 2: Full Config Stack + NewRepositoryWithProperties ✅ DONE

Commit: `feat(registry): use full config stack and NewRepositoryWithProperties`

### What was added

- `config.LoadConfigs()` called in `NewClient()` to load:
  - Docker `config.json` (credentials)
  - Containers `auth.json` (Podman/Buildah credentials)
  - `registries.conf` (mirrors, blocked registries, insecure flags)
  - `certs.d` (per-registry CA + client certificates)
  - `policy.json` (loaded, ready for policy enforcement)
  - `registries.d` (signature lookaside config)
- Combined credential store: helm store (primary, for Login/Logout) → Docker config → containers auth
- `newRepository(ref)` uses `configs.RegistryProperties(ref)` + `NewRepositoryWithProperties(props, builder)`:
  - Resolves mirrors from `registries.conf`
  - Applies per-registry TLS from `certs.d`
  - Applies CLI flag overrides via `applyOverrides()`
- `newRegistry(host)` (Login path) manually applies `registries.conf` insecure flag and `certs.d`
- `ClientBuilder` configured with combined credential store, user-agent, and debug logger
- `GenericClient` refactored to hold `*Client` reference and delegate to `newRepository()`
- Legacy path preserved when `ClientOptHTTPClient` or `ClientOptAuthorizer` is used (e.g. in tests with custom TLS transport)

### CLI flag override support

The following flags are applied on top of config-driven settings:

| Flag | Field set | Applied in |
|------|-----------|------------|
| `--plain-http` | `c.plainHTTP` | `applyOverrides()` → `props.Transport.PlainHTTP` |
| `--insecure` | `c.insecure` | `applyOverrides()` → `props.Transport.Insecure` |
| `--cert-file` / `--key-file` | `c.certFile`, `c.keyFile` | `applyOverrides()` → `props.Transport.Cert/Key` |
| `--ca-file` | `c.caFile` | `applyOverrides()` → `props.Transport.CACerts` |
| `--username` / `--password` | `c.username`, `c.password` | `applyOverrides()` → `props.Credential` |

---

## Part 3: Policy Enforcement ✅ DONE

Wire `policy.json` into the `ClientBuilder` so all pull operations are
validated before fetching.

### Tasks

1. Add `PolicyEvaluator *policy.Evaluator` to `Client` struct
2. In `NewClient()`, build the evaluator if `configs.PolicyConfig != nil`:
   ```go
   evaluator, err := configs.PolicyEvaluator()
   client.policyEvaluator = evaluator
   ```
3. Set `builder.PolicyEvaluator = client.policyEvaluator` before building repositories
4. Add `ClientOptPolicyEvaluator(e *policy.Evaluator)` option for override
5. Tests: add a test registry with a deny-all policy and verify pulls are rejected

### Relevant packages
- `github.com/oras-project/oras-go/v3/registry/remote/policy`
- `configs.PolicyEvaluator(opts ...policy.EvaluatorOption) (*policy.Evaluator, error)`

---

## Part 4: Signature Verification ✅ DONE

Verify image signatures using `registries.d` lookaside storage and
`policy.json` `signedBy` rules.

### Tasks

1. After building the policy evaluator, wire in signature verification:
   ```go
   evaluator, err := configs.PolicyEvaluator(
       policy.WithSignedByVerifier(
           signature.NewSignedByVerifierFromConfig(configs.RegistriesDConfig, scope),
       ),
   )
   ```
2. `scope` is the registry/repository being accessed (e.g. `registry.example.com/app`)
3. The evaluator returned is set on `builder.PolicyEvaluator`
4. Add `ClientOptSignatureVerification(enabled bool)` option

### Relevant packages
- `github.com/oras-project/oras-go/v3/registry/remote/policy`
- `github.com/oras-project/oras-go/v3/signature`
- `configs.RegistriesDConfig` — loaded automatically by `LoadConfigs()`

---

## Part 5: LoadConfigsWithOptions Override Paths ✅ DONE

Expose `LoadConfigsWithOptions` so callers can override specific config file
paths (e.g. a non-default `registries.conf` or `policy.json`).

### Tasks

1. Add fields to `ClientOption` or a new `ConfigOptions` struct:
   ```go
   type ConfigOptions struct {
       RegistriesConfigPath string
       PolicyConfigPath     string
       CertsDirPaths        []string
       ContainersAuthPath   string
   }
   ```
2. Add `ClientOptConfigOptions(o ConfigOptions) ClientOption`
3. In `NewClient()`, use `remoteconfig.LoadConfigsWithOptions(...)` when overrides are set
4. Expose via Helm action flags (e.g. `helm install --registries-config /path/registries.conf`)

---

## Part 6: Login Path — Mirror and Rewrite Support ✅ DONE

`newRegistry()` (used by `helm registry login`) currently only applies the
`Insecure` flag from `registries.conf`. Mirrors and `Location` rewrites are
not applied on login.

### Tasks

1. Investigate whether login should follow mirrors (probably not — login targets
   the primary registry, not a mirror)
2. Apply `Location` rewrite from `registries.conf` if present so the login
   target is the canonical registry name
3. Add integration test: login to a registry that has a `Location` rewrite in
   `registries.conf` and verify the stored credential key matches the rewritten host

---

## Remaining Items Summary

| # | Description | Status |
|---|-------------|--------|
| 1 | Mechanical v2 → v3 migration | ✅ Done |
| 2 | Full config stack + NewRepositoryWithProperties | ✅ Done |
| 3 | Policy enforcement (policy.json) | ✅ Done |
| 4 | Signature verification (registries.d + signedBy) | ✅ Done |
| 5 | LoadConfigsWithOptions override paths | ✅ Done |
| 6 | Login path mirror/rewrite support | ✅ Done |
