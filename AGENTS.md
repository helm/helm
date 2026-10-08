# AGENTS.md

## Overview

Helm is a package manager for Kubernetes written in Go. It enables users to define, install, and upgrade complex Kubernetes applications using charts.
This document provides an overview of the codebase structure, development guidelines, and key patterns for contributors.

The codebase supports both an SDK for advanced users, and a CLI for direct end user usage.

The project currently supports Helm v3 and Helm v4 versions, based on the `dev-v3` and `main` branches respectively.

## Build and test

```bash
make build              # Build binary
make test               # Run all tests (style + unit)
make test-unit          # Unit tests only
make test-coverage      # With coverage
make test-style         # Linting (wraps golangci-lint)
go test -run TestName   # Specific test
```

## Code structure

Major packages:

- `cmd/helm/` - CLI entry point, wires CLI flags to `pkg/cmd/` commands
- `pkg/` - Public API
  - `action/` - Core operations (install, upgrade, rollback)
  - `cmd/` - Cobra command implementations bridging CLI flags to `pkg/action/`
  - `chart/v2/` - Stable chart format
  - `engine/` - Template rendering (Go templates + Sprig)
  - `kube/` - Kubernetes client abstraction layer
  - `registry/` - OCI support
  - `release/` - Release types and interfaces (`v1/`, `common/`)
  - `repo/` - Chart repository indexing and interaction
  - `storage/` - Release backends (Secrets/ConfigMaps/SQL)
- `internal/` - Private implementations
  - `chart/v3/` - Next-gen chart format
  - `release/v2/` - Release package for chart v3 support

## Development

### Code standards

- Use table-driven tests with testify
- Golden files in `testdata/` for complex output
- Mock Kubernetes clients for action tests
- All commits must include DCO sign-off: `git commit -s`

### Branching

Standard workflow is for PR development changes to the `main` branch. Minor release branches are cut from `main`, then maintained for critical fixes via patch releases.
Bug and security fixes are also backported to `dev-v3` where applicable.

Development branches:

- `main` - Helm v4
- `dev-v3` - Helm v3 (backport security and bugfixes from main)

Release branches:

- `release-v3.X` - Release branches for v3.X versions
- `release-v4.X` - Release branches for v4.X versions

### Major dependencies

- `k8s.io/client-go` - Kubernetes interaction
- `github.com/spf13/cobra` - CLI framework
- `github.com/Masterminds/sprig` - Template functions

### Key patterns

- **Actions**: High-level operations live in `pkg/action/`, typically using a shared Configuration
- **Chart versions**: Charts v2 (stable) in `pkg/chart/v2`, v3 (under development) in `internal/chart/v3`

---

# Helm Project Philosophy - Agent Operating Guide

An operating guide for an AI working on Helm (`helm/helm`, `helm/community`, charts,
docs). It encodes *how Helm maintainers think* so a proposed change can be judged the way
they would judge it, before writing code, opening an issue, or filing a HIP.

It is not the contribution mechanics (build, test, DCO, PR flow). It is the decision lens
that sits above those.

## How to use this guide

- Treat every rule as a **default with a stated reason**, not a law. When two rules
  conflict, the Prime Directives win, and you say so and explain the tradeoff.
- Rules are grounded in what maintainers actually said. Citations are inline as
  `- who, source (date)`; full source list is at the end.
- **Matt Farina's stated views are weighted heaviest.** He is a long-time core maintainer,
  an org maintainer, and the Helm 4 Release Engineer, and is by a wide margin the
  most-quoted design voice across the developer calls, GitHub, HIPs, blogs, and talks. His
  technical blog is `codeengineered.com` (not `mattfarina.com`, a profile page).
- When unsure whether something is "Helm-appropriate," run it through the
  change-proposal checklist.

## The through-line

If you internalize one frame, make it Farina's: **Helm is a package manager that does one
thing well; it packages an expert's knowledge behind a chart's parameter interface so a
non-expert can install, run, and succeed; it earns trust by never breaking users and never
mutating cluster state it does not own; and it owns every tradeoff explicitly.**

- "a package manager takes... you've got knowledge... on an application like Postgres...
  and combine it into a package... so somebody who doesn't know any of that... can install
  it and it works and it's simple." - Farina, KubeCon NA 2019 (ASR)
- "it is a package manager and we draw our boundaries there, but by drawing our boundaries
  and knowing what our boundaries are, that means other projects can pick up from it." -
  Farina, KubeCon NA 2022 (ASR)
- "we work very hard to not break users. Move fast and break things isn't what Helm is
  about because it's a building block to other things." - Farina, helm/helm#5871 (2020)

---

# Engage by guiding, not dictating

This is the first principle because it shapes how every other principle is delivered. Helm's maintainers steward a large community of chart authors and operators, most of whom are solving a real problem when they open a PR. The job of review is to reach a better outcome and keep the contributor willing to come back, not to win a point or close a tab.

In practice:
- Assume the contribution answers a real need. Ask what problem it solves before judging the solution.
- When something does not fit, explain why, and point to the path that does: a plugin, a post-renderer, values, or a HIP for a cross-cutting change. A bare "no" is not a review.
- Prefer a question that surfaces the tradeoff over a verdict. "What happens to existing charts that rely on the current behavior?" teaches more than "this breaks compatibility."
- Cite the principle or the precedent, not just the conclusion. A contributor cannot apply a rule they cannot see.
- Lower the barrier for humans. The maintainers' concern with automated review has been volume and slop, not tooling; feedback should reduce friction for a good contribution, not add a gate.

For an agent reviewing a PR, this means: raise these principles as prompts for a human reviewer and as guidance for the author, never as automated rejections. Tone is not a nicety here. It is the difference between review that helps and review that drives contributors away.

The field notes in `philosophy-appendix.md` (the values-templating and post-render refusals, the automated-review discussion) show this posture in maintainers' own words.

---

# The change-proposal checklist

Run any proposed change through these. A "wrong" answer is not an automatic veto; it flags
that you owe an explicit, stated tradeoff (and probably a HIP).

1. **Compatibility.** Does it change a command, flag, flag type, structured output, Go SDK
   signature/interface, `Chart.yaml` field, or remove a template function or public symbol?
   If yes it is breaking -> major-version / HIP track / new chart apiVersion, not a minor.
   Only... actually, not even security justifies a silent user break. Forward compat too:
   can older Helm still load the chart?
1. **Scope.** Package management, or release-orchestration / CD / config-management that
   belongs elsewhere or in a plugin? Could it be a second chart or a post-renderer? "Does
   this belong in Helm?"
1. **Safety.** Does it auto-manage/delete/mutate cluster state Helm did not create (CRDs,
   foreign resources, live objects)? Refuse or require explicit manual intent. Is rendering
   deterministic, self-contained, and free of remote I/O?
1. **Complexity / ownership.** Does it encapsulate complexity behind the chart API, or expose
   more? Does it add a dependency, subsystem, or a second way to do something the maintainers
   must own forever? Fix the specific bug, not a generic expansion.
1. **Roles.** Serves the chart-consumer role? Or assumes the installer knows internals or is
   a human at the CLI (not CI/Flux/SDK)? Holds up on the unhappy path and multi-tenancy?
1. **SDK.** Does it return errors (not log/panic/exit)? Injectable output (default
   `io.Discard`)? Logs to stderr, output to stdout? Minimal surface? It carries the same
   compat guarantee as the CLI.
1. **Portability / fragmentation.** Would a chart depend on an environment-specific engine,
   plugin, or post-renderer? Cross-platform, Windows included?
1. **Security.** Standing in-cluster privilege, a central server, a new trust boundary,
   auto-pulling unadded sources, plaintext by default? Least privilege, safe-default +
   opt-out flag, never decrease supply-chain security. Flag for human security review.
1. **Consistency.** Reuses existing mechanisms/syntax, least astonishment, ecosystem idioms?
   Considered all three config surfaces (flag / annotation / env)?
1. **Tradeoff.** Can you state the disadvantage as clearly as the advantage, priced in cost
    to thousands of downstream users?
1. **Process.** Major or cross-cutting? HIP with a backward-incompatibility section,
    consensus, maintainer approval; new/unproven -> start as a plugin. Tests as a merge gate;
    main-first, backport-after.
1. **Longevity.** Adds churn / migration friction? Is the novelty worth what it costs
    existing users? Is there a simpler, low-fi solution?

---

# Tier 1: Does this belong, and does it change behavior

## Prime Directives

These override every other rule.

1. **Do not break users.** Within a major version, minor and patch releases must be 100%
   backward compatible (CLI, structured output, Go SDK API, `Chart.yaml`, template
   functions). Helm is a building block; a break cascades. See Backward compatibility.
1. **Stay a package manager, and do one thing well.** Helm packages, distributes, installs,
   and tracks releases of Kubernetes resources. Compose with other tools; do not absorb
   them. See Scope.
1. **Never auto-manage cluster state Helm does not provably own.** CRDs, foreign resources,
   live objects: safety over convenience, every time. A partial fix that risks data loss is
   worse than an open issue. See Resource lifecycle.
1. **Encapsulate complexity behind the chart API; design for the consumer role.** The chart
   author holds the expertise; the consumer supplies parameters and should need little
   Kubernetes knowledge. See The two-role model.

---

## Scope: what Helm is, and what it is not

**Rule: Frame Helm as a package manager in the apt / yum / zypper / homebrew lineage, and
invoke the Unix philosophy - do one thing well.** Fewer responsibilities also means less
code to own.
- "Helm is a package manager and we, to a large extent, like the Unix philosophy from Ken
  Thompson... we want to focus on it doing the one thing well." - Farina, helm/helm#8453 (2020)
- "Helm is a package manager: we know our space, we're not trying to scope-creep to areas
  where other tools are doing a great job." - Farina, The Stack (2025)

**Rule: Release management, ordering, config management, and "how instances run" are out of
scope.** Work well with those tools; do not become them.
- "This is release management and beyond scope of a package manager." - Farina, dev-call
  notes (2021-08-05)
- "This request sounds too outside of the scope of helm to me. Other approaches might work
  like putting TLS certs and CRDs in another chart." - Howe, helm/helm#30993 (2025)

**Rule: Helm is a templating/package tool, not an operator; Kubernetes is declarative, so
imperative orchestration belongs in an operator.** The mental model is
`helm template ... | kubectl apply -f -`; complex runtime logic is repeatedly rejected.
- "It's generally rejected to add any complex logic, the argument being that helm is a
  templating tool, not an operator." - Joe Julian, helm/helm#11359 (2022)
- "Kubernetes is a declarative system. When you need imperative processes, that's usually
  when you want to create an operator." - Joe Julian, helm/helm#6283 (2023)

**Rule: Compose rather than grow.** Helm already manages plain manifests; reach for an
existing tool as a post-renderer before adding to core.
- "you can use Helm to manage the release lifecycle of plain k8s manifests today... You can
  even use kustomize as a Helm postrenderer." - Rigby, helm/helm#31167 (2025)

**Rule: Add-on capability is a second chart or an external tool, not core/chart bloat.**
- "why wouldn't the extra objects be a second chart someone installs... Consider package
  managers for other platforms (like Linux)." - Farina, helm/helm#12653 (2023)

**Rule: Cover the ~95% case well; do not distort the tool for every edge, and do not make
Helm opinionated.** There is no single right way; codifying one workflow excludes real
users, and Helm holds a near-monopoly position.
- "we try to be as non-opinionated as possible... each of you probably has at least one
  workflow you'd like to do and many of them are not gonna overlap, so how can we not get in
  the way of that?" - KubeCon NA 2018 (ASR, unlabeled)
- "There isn't one right way to do things... This is why Helm is not more opinionated." -
  Farina, helm/helm#9791 (2021)

**Rule: In the AI-contribution era, review is increasingly a scope question.** "Does this
belong in Helm?" is now the primary review lens, not mechanics.
- "it's starting to come down to where it's more of a philosophical review. It's like, does
  this actually belong here or not?" - developer call 2026-07-09 (ASR)

> No fetched source enumerates non-goals as a negative list. Argue scope from the
> package-manager identity, not from a non-goals page that does not exist.

---

## Backward compatibility and versioning

The flagship. Read HIP-0004 before touching a public surface.

**Rule: Assume any change to a public surface is breaking until proven otherwise.**
HIP-0004 makes minor/patch releases 100% backward compatible: commands and flags must not
be removed, renamed, moved, repurposed, or change type; structured-output format must not
change; template functions cannot be removed; new fields must be optional; the Go SDK must
keep compiling. (HIP-0004: Khouzam and Butcher, accepted 2020-09-18.)
- "we take that backwards compatibility maybe too seriously... people parse the output
  strings from Helm, and if you go to parse it and there's characters in there to do color,
  then we can break your interface. And so we waited a long time to put color in." - Farina,
  KubeCon NA 2025 (ASR)
- "The price of a true SemVer policy is eternal vigilance." - Khouzam, helm/helm#7862 (2020)

**Rule: Follow Go's (deliberately painful) compatibility discipline - add a new interface,
never mutate a released one.** In Go, even *extending* an interface breaks implementers.
- "once you create an interface and you release it, you never change that until the next
  major version... Instead, you create a new interface and then you type switch to that
  interface. And we followed guidelines... that are sometimes painful." - Farina, KubeCon NA
  2025 (ASR)
- "In golang, extending an interface is a breaking change because any type implementing that
  interface will break." - Mungai, helm/helm#30697 (2025)

**Rule: A breaking change is deferred to the next major, or gated behind a new chart
`apiVersion` - never smuggled into a minor, and never a silent behavior change.** This holds
even for bugfixes and "corrections."
- "both the scheme and path will be stripped automatically... in order to keep Helm's
  backwards compatibility promise within the same MAJOR version." - Rigby, helm/helm#30873 (2025)
- "changing behavior for current charts (apiVersion v2) could cause unintended effects...
  Chart apiVersion v3 can make behavior changes like this." - Rigby, helm/helm#12265 (2026)
- "we wanted ways to create a future where we could actually get to some of these more fun
  features without breaking people, because we understand people will hunt us down if we
  break you." - Farina, KubeCon NA 2025 (ASR)

**Rule: Changing structured output, removing a public Go symbol, or extending an interface
are all breaking.** Even a version-string format is arguably output.
- "Changing the output format is considered a breaking change according to hip-0004." -
  Mungai, helm/helm#31574 (2025)
- "Since this is not an internal package, this probably needs to stay public or this would
  be a breaking change." - Howe, helm/helm#13185 (2024)

**Rule: A behavior change must be opt-in - add a flag, or make the annotation itself the
opt-in; never change a default.** Users scrape Helm's table output, so even a helpful new
column is breaking (extend the JSON/YAML output instead).
- "Changes of behavior are precluded without opting in by hip-0004." - Joe Julian, helm/helm#7874 (2023)
- "My thought on this is that the act of adding the annotation is the opt-in." - Joe Julian,
  helm/helm#8132 (2022)
- "A lot of folks scrape the output of helm and parse the text... the table output would be
  considered breaking. It could still be added to the json and yaml outputs." - Joe Julian,
  helm/helm#11326 (2023)

**Rule: Forward compatibility constrains you too.** A strict schema means you can never add
a field within an `apiVersion` without breaking older Helm that is still in wide use.
- "it means we will never be able to add another field to the Chart.yaml file in the current
  apiVersion. Older versions of Helm... will fail to load the chart." - Farina, helm/community#371 (2024)

**Rule: A security improvement does not justify silently breaking users.** Even a
security-audit finding waits if the fix breaks charts/workflows.
- "the to-do was to go in and do that. And that was a breaking change to users. So, rightly
  so, we never made that change." - Farina, developer call 2025-10-02 (ASR)

**Rule: Respect the Kubernetes support window and make no forward-compat guarantee.** The
window covers public-cloud stable versions (Farina argued n-2 over n-1 in 2019 so no cloud
provider is excluded; current published Helm 4 policy is n-3).

**Rule: Ship security fixes as patch releases, always.** Shipping one only in a minor drew
community backlash. - Fisher and Farina, dev-call notes (2020).

---

## Keep the core small: maintenance-burden minimalism

**Rule: Do not adopt a dependency or subsystem to serve one feature; the cost of ownership
is the gate.** Point requesters at a plugin or wrapper.
- "There is not enough maintainers to maintain both a package manager AND a YAML parser." -
  Fisher, helm/helm#3141 (2020)
- "I don't think bringing in all these dependencies is a good idea... helm should [not]
  integrate directly with storage solutions unless it was through some plugin type
  architecture." - Howe, helm/helm#12173 (2025)

**Rule: Fix the specific bug; do not build a generic solution that expands scope.**
- "I changed the implementation to ignore .git when installing plugins. This addresses the
  bug instead of attempting to implement a generic solution." - Mungai, helm/helm#31250 (2025)

**Rule: Do not maintain two ways to do the same thing** (merge strategies, dual sources of
truth, parallel code paths). Each multiplies edge cases.
- "maintaining multiple merge strategies in Helm will lead to a completely different set of
  edge cases and would be a significant increase in maintenance cost." - Fisher, helm/helm#3805 (2018)

**Rule: Reject powerful-but-leaky features whose edge cases become an endless bug queue.**
- "using full-fledged templates is a major undertaking, and even if successful it would
  result in an interminable series of bug reports filed when people hit the edge cases." -
  Butcher, helm/helm#2492 (2017)

**Rule: Extensibility is how the core stays small AND maintainable.** Plugins let
contributors extend Helm without core changes; flag proliferation is the symptom that need
creates.
- "Helm is currently a monolithic application that is difficult to customize without
  changing the core codebase. This requires maintainers to review and accept every
  contribution, which is... not scalable... making Helm not only more extensible, but also
  more maintainable." - Rigby and Jenkins, HIP-0026 (2025)

---

## Charts, templating, values, and determinism

**Rule: Keep Go templates plus Sprig as the engine; templating exists to eliminate
duplication and give conditionals/iteration that plain parameterization cannot.**
- "mere parameterization didn't work" when you need "different structures, not simply a
  string substitution." - Butcher, SE Radio 509 (2022)
- "it prevents that you're duplicating the same code everywhere." - Dolitsky, KubeCon NA 2019 (ASR)

**Rule: Do not add pluggable/alternative template engines to core (helm/helm#2577, #6184).**
The rejection reasons are the template for any extensibility request: fragmentation of which
Helm can install which chart, dependency-hell UX across the chart tree, low real demand
(Helm 2's `EngineYard` hook went unused), trust/supply-chain risk, and cross-platform
(Windows) portability. - Farina and Butcher, helm/helm#6184 (2019). In Helm 4, a *mixable*
alternative (YAMLScript) is explored *within* the chart via the plugin system, not as a core
engine swap: "you can mix the two at any level... I don't want to imply that you have to
write your entire chart in YAMLScript." - developer call 2024-12-20 (ASR).

**Rule: Charts should be functionally pure - same inputs, same output - and rendering must
be deterministic and self-contained.** This is the stated ideal that governs what may touch
templating.
- "charts should... always produce the same value for the same inputs... functionally pure
  would be ideal. They're definitely not today." (`lookup`, random/`uuid`/clock, and
  post-renderers are named as what breaks it) - developer call 2024-12-20 (ASR)
- "Post-rendering solutions violate Helm's design philosophy that template rendering should
  be deterministic and self-contained... keeps upgrade logic in the chart itself,
  maintaining Helm's portability, testability, and transparency." - HIP-0029 (2025)
- "It is intentional behaviour for lookup to return with an empty dictionary during a helm
  template, as it is expected that the chart renders without any cluster connection." -
  Fisher, helm/helm#8137 (2020)

**Rule: No remote I/O during templating - it is a security boundary.** DNS lookups are
disabled by default to prevent exfiltration.
- "we don't enable DNS lookups by default, we disable DNS lookups... getHostByName - you
  could do DNS exfiltration to send out someone's secrets like AWS credentials... we have a
  security boundary where we don't... interact with remote sources, especially not in the
  templating phase." - developer call 2024-12-20 (ASR)

**Rule: `values.yaml` must always be valid YAML in its raw state.** Values are data, merged
in stages (some before the engine exists: `--set`, `-f`, dependency constraints).
- "The bigger constraint is that the values.yaml file MUST always be a valid YAML file." -
  Butcher, helm/helm#2492 (2017)

**Rule: Treat `--set` as the binding constraint the values file is designed around** - favor
flat over nested, maps over arrays; document every property (name-first, so tooling can
correlate); quote all strings (YAML implicit coercion is the hazard); begin value names
lowercase (initial caps collide with built-ins). - Chart Best Practices: Values.

**Rule: Schema validation (`values.schema.json`) is a chart's input contract,
un-bypassable by a parent.** Conceived for both validation and form generation.
- "The goal of this to have schema files for values.yaml files. This can be used for
  validation and the generation of forms." - Farina, helm/helm#5081 (2018)

**Rule: Subcharts are isolated; sharing is by explicit exception only.** A subchart cannot
read up into its parent; `.Values.global` is the narrow deliberate crack; enable/disable
(`condition`/`tags`) flows top-down from the top parent.
- "I'm very reluctant to allow subcharts to modify parent charts without 'explicit consent'
  because of the potential for collisions and unintended side effects." - Butcher, helm/helm#1883 (2017)

**Rule: Dependencies resolve, pin, and vendor at build time - zero runtime ambiguity.**
Chart metadata is self-contained in `Chart.yaml` (`dependencies`, `Chart.lock`) for
apiVersion v2. - Butcher, SE Radio 509 (2022).

**Rule: Charts stay universally installable - never let a chart depend on
environment-specific extension** (a specific post-renderer or engine plugin). Post-rendering
is a CLI escape hatch, not a `Chart.yaml` field. - Thomas, helm/helm#7260 (2020). Keep
library charts non-installable.

---

## Extensibility: plugins, experimental features, WASM

**Rule: Plugins are the pressure valve that lets the core stay small; prefer a plugin to an
in-core "experimental" mode** (experimental confused users). In Helm 4, built-ins move out:
post-renderers become plugins.
- "instead of experiments, we should look at plugins in the future... plugins will be a more
  clear delineation." - Farina, dev-call notes (2021)
- "In Helm 4, with post-renderer now being a plugin type, you can still call a binary, you
  just need to define the binary in a plugin file." - Rigby, helm/helm#31340 (2025)

**Rule: Refuse bespoke in-core integrations - this is the clearest statement of the plugin
boundary.** A specific integration in core forces Helm to standardize one workflow for
everyone, grows more elaborate over time, gets frozen by the backward-compat guarantee, and
must be maintained "forever."
- "we (Helm maintainers) want to avoid such specific integrations. They force Helm to: a)
  standardize the workflow for everyone... b) the integrations begin to become more
  elaborate over time... c) are difficult to change/evolve (... subject to Helm's backwards
  compatibility guarantees), d) burdensome to maintain and support: the Helm maintainers
  must support the feature 'forever'." - Jenkins, helm/helm#32258 (2026)

**Rule: Once an experimental feature goes GA, its API freezes until the next major.** -
Farina, dev-call notes (2021).

**Rule: Helm 4 plugins are sandboxed WebAssembly - build once, run anywhere.** This kills
per-platform maintenance, isolates untrusted code, and is the sandbox that could finally make
post-rendering safe. Design the runtime behind an adapter so it can be swapped. SDK users
link Go libraries directly rather than using WASM.
- "this is a strong reason to go with WASM because it provides the sandbox that we need...
  the goal would be probably to kill off the post-renderer flag." - developer call 2024-12-20 (ASR)
- "Agree on designing with an adapter pattern in mind [so the Wasm runtime could be
  swapped]." - Rigby, helm/community#388 (2025)

**Rule: A mature community plugin can be adopted into the org via a defined vote** (e.g.
mapkubeapis). - Butcher, helm/community#157 (2021).

---

# Tier 2: Design principles

## The two-role model: chart author vs chart consumer

**Rule: Design for the consumer who supplies parameters, not the author who wrote the
templates.** A package lets an expert encapsulate knowledge so a non-expert succeeds.
- "it's the chart creator who creates the templates, but the chart consumer doesn't change
  them. The chart consumer only works with the parameters they pass in." - Farina, SE Radio
  509 (2022)
- "Problems can arise when package authors start to assume that end users know nearly as
  much as they do." - Farina, helm/helm#10026 (2021)

**Rule: The persona split is a design constraint, not a nicety - it is why Helm diverges
from `kubectl`.** Chart developers cannot know which fields another controller manages in a
consumer's cluster.
- "Dissimilar to kubectl, Helm distinguishes between chart developers and chart operators.
  Chart developers may not consider, or may not even know, which fields may be overwritten
  by another process." - Jenkins, HIP-0023 (2023)

**Rule: Design for the unhappy path, multi-tenancy, and non-human installers.** Chart
authors do not control installers; two unaware tenants share a cluster; CI and Flux install
charts too; non-experts file the data-loss issues.
- "Application operators often do not have expertise in k8s... When an application operator
  has a problem, especially a severe one like data loss, they file issues in the Helm issue
  queue... one of the reasons Helm has been conservative." - Farina, helm/community#379 (2025)
- "We cannot assume that the thing installing a chart is a person using the Helm CLI. It
  could be happening in CI or via a system like Flux." - Farina, helm/community#301 (2023)

---

## Resource lifecycle: CRDs, hooks, merges, ownership

**Rule: Correctness and safety beat convenience - never silently mutate state Helm did not
create.** Ask for manual intervention rather than guess; unexpected deletion is a data-loss
bug, not a config choice.
- "we automatically roll back and delete resources... This is very risky as the cluster may
  be in an unknown state... Helm may delete objects that were installed via other
  packages... The safest option so far has been to ask users to manually intervene." -
  Fisher, helm/helm#1193 (2018)
- "I don't like things being deleted by surprise... 'I don't want this thing to be deleted'
  and it gets deleted, that's a bug - data loss." - Joe Julian, developer call 2025-05-01 (ASR)

**Rule: Helm installs CRDs but does not upgrade or delete them.** CRDs are cluster-global;
deleting one deletes every custom resource of that kind across all tenants. Read HIP-0011.
Templated CRDs in `templates/` remain valid (a documented reason: conversion webhooks - Joe
Julian, dev-call notes 2021). The community-preferred pattern is CRDs in their own chart
that owns update/migration.
- "Automatic deletion of CRDs is a serious no-no... (I was the author of crd-install, and I
  am very dissatisfied with it.)" - Butcher, helm/helm#6243 (2019)
- "I've always been a fan of shipping crds in their own chart and that chart handles
  updating and migrating." - Howe, helm/helm#30600 (2026)

CRD management is an acknowledged real gap, stated candidly, not a solved problem:
- "we (D2iQ) had to fork the cert-manager chart (and a bunch of others) because of how it
  handles CRDs. Istio has abandoned helm and developed their own installer binary because of
  CRD management. I agree... CRD management is a problem that needs to be addressed." - Joe
  Julian, helm/helm#5871 (2020)
- "Helm does not have any way of parsing CRDs and comparing a resource with the definition
  of that CRD." - Joe Julian, helm/helm#10869 (2022)

**Rule: On CRD upgrades, fail fast rather than silently merge or silently skip - a
mishandled storage/conversion version loses or corrupts data, and silent fall-through is
worse in air-gapped/compliance environments.** Keeping a deprecated CRD version around is
unsafe if the conversion webhook no longer supports it.
- "This won't work if the version of the webhook that gets installed doesn't support the
  conversion... the data won't be lost, but it may not be supported by the conversion
  webhook nor the controller." - Joe Julian, helm/community#379 (2025)
- "my preferred behavior would be that instead of merging CRDs, that if the storage version
  in the installed CRD is no longer in the new CRD version, we just error and fail." - Joe
  Julian, helm/community#379 (2025)
- "silently skipping it can cause Helm to fall through to the original upstream registry
  instead of honoring the intended policy... that is worse than failing fast." - Joe Julian,
  helm/community#391 (2026)

**Rule: Solve the hard part of a problem or do not merge it.** Partial CRD solutions that
handle only create get rejected; Helm must consider more than one path.
- "The Create step in the CRD CRUD is the easiest to solve. It's the RUD parts that are more
  complicated." - Farina, helm/helm#5871 (2019)

**Rule: Do not grow hooks into a workflow engine.** Hooks create unmanaged objects and do
simple ordering; complex workflow belongs in a separate tool. Prefer a narrow annotation
over a specialized code path. - Butcher and Fisher, helm/helm#2243 (2017).

**Rule: Reconcile against live cluster state, do not clobber out-of-band edits or nuke
state.** Helm invented three-way strategic merge (old manifest, live state, new manifest) so
`kubectl edit` changes and injected sidecars survive upgrade; it must not delete a
PersistentVolumeClaim on upgrade. Release state is a namespaced Secret; the storage backend
is a pluggable driver.
- "Helm came up with a three-way strategic merge patch. Kubernetes saw this and built it
  into kubernetes" (as server-side apply). - developer call 2024-12-20 (ASR)
- "heavens forbid that you delete your persistent volume [claim]... most people... don't want
  to have their application's state deleted when they do an upgrade." - KubeCon EU 2019 (ASR)

**Rule: Adopt Server-Side Apply in Helm 4 to delegate field management to Kubernetes, opt-in
by default, but never silently switch an existing release.** Delegation also lets Helm
eventually drop its own merge code. Preserve prior behavior: reuse the previous release's
choice.
- "Helm should adopt SSA... it is unlikely client-side methods will continue to be improved
  upon... may eventually allow Helm to drop... the strategic-merge patch CSA
  implementation." - Jenkins, HIP-0023 (2023)
- "if a chart worked with server-side apply previously... you would continue to manage it
  with server-side apply." - developer call 2024-12-20 (ASR)

**Rule: Do not make the cluster a source of truth, and delegate client/version concerns to
`client-go`.** Treating the cluster as canonical state turns it into a stateful "pet";
version-skew is Kubernetes' domain, not something Helm papers over.
- "I would recommend against using a Kubernetes cluster as a source of truth, as that makes
  the cluster a stateful 'pet'." - Jenkins, helm/helm#32258 (2026)
- "there isn't much Helm can do to help here, unless your error occurs only with Helm (and
  not kubectl) and on a supported Kubernetes version." - Jenkins, helm/helm#31911 (2026)

---

## Distribution and trust

**Rule: Distribution is decentralized by design - Helm distributes, it does not host or
curate.** Repositories were meant to be many; discovery is an aggregator's job (Artifact
Hub), not a gatekeeping team's. The central `helm/charts` monorepo failed because one team's
workflow could not scale.
- "People who wanted to maintain their charts... all had to use one workflow that we came up
  with... They had to wait on charts maintainers... frustrating for both them and us." -
  Farina, QCon 2020
- "As you wanted to scale up without burning out your maintainers... you add automation...
  That was the only way we could have scaled." - Farina, QCon 2020

**Rule: Prefer a standard substrate over a bespoke protocol.** OCI won as a common API for
all package types that reuses existing registry infrastructure - not for a Docker-like CLI.
Accept the loss of central search as the price. Register media types with IANA so they are
externally discoverable.
- "The true value of leveraging OCI specs has very little to do with the command-line
  experience... OCI registries provide a common API for all types of packages." - Dolitsky,
  HIP-0006 (2020)
- "each type should be registered with IANA so that it can... be discoverable by end users."
  - Block, HIP-0017 (2022)

**Rule: An OCI registry is a genuinely different model, not a repo-index rename; chart
identity is the SemVer version; distribution tooling helps authors, it does not enforce.**
- "in Helm registries don't currently have an equivalent to a helm repo or indexing concept
  across multiple charts." - Rigby, helm/helm#10553 (2022)
- "not as an enforcement (you could bypass this easily...), but as a helper for helm chart
  authors to follow best practices." - Rigby, helm/helm#11736 (2025)

**Rule: The Helm project refuses to be a mandatory trust root; trust must be provable.**
Signing keys are distributed out of band, so a compromised registry cannot forge trust
(why OpenPGP was chosen). Sign at package, verify at install/pull.
- "the Helm project would not insert itself into the chain of trust as a necessary party...
  we strongly favor a decentralized model, which is part of the reason we chose OpenPGP." -
  provenance docs
- "If you're getting something from... Bitnami, did you really get it from them?... can you
  trust it?" - Farina, QCon 2020

**Rule: Helm does not host repositories.** `helm serve` was removed (unused; files are
already on disk); hosting is delegated to ChartMuseum or a plugin. - Butcher, helm/helm#7584 (2020).

---

## Security posture

**Rule: Security is layered, not boolean, and supply-chain security may only increase.**
Do not accept a change that lowers it for convenience. - Farina, helm/helm#9903 (2021).

**Rule: Never reintroduce a privileged in-cluster server or central hub.** Deleting Tiller
was the defining Helm 3 decision; Helm is client-side and inherits the user's Kubernetes
identity and RBAC. Tiller was effectively root and could not be locked down.
- "It was very, very hard to lock down Tiller so that you couldn't have people install all
  kinds of things... as... the root user." - Butcher, SE Radio 509 (2022)

**Rule: Default to least privilege, and make the safe behavior the default with an opt-out
flag.** Dependency sources must be explicitly opted into, like any package manager.
- "normally with package managers, you'd say 'I opt into that one. I've checked it out. I
  validated it.'... And Helm didn't do that." - Farina, developer call 2025-10-02 (ASR)
- "I would just put this check in by default and then give you a flag to disable it just so
  it's always there." - Farina, developer call 2025-10-02 (ASR)
- Allow-list over deny-list, at the right layer (a plugin): a deny-list "is infinite" and
  bad UX. - developer call 2025-10-02 (ASR)

**Rule: Secure by default.** Require explicit opt-in for plaintext HTTP to a registry; keep
decompression size limits on; missing provenance warns rather than hard-fails (mirroring
chart `--verify`); keep a pluggable path to newer signing (sigstore).
- "--plain-http is now required to communicate with a registry that only supports http. From
  a security perspective, this is the way it should work." - Howe, helm/helm#13382 (2025)
- "The MaxDecompressedFileSize limit is meant to set secure by default." - Rigby, helm/helm#31748 (2026)
- "updates the behavior for installing a plugin with missing .prov file to now emit a
  warning and continue, instead of failing." - Rigby, helm/helm#31176 (2025)

**Rule: Threat-model concrete attacks; do not assume users read the docs; treat untrusted
charts as hostile input** (env-var exfiltration, zip bombs, DNS exfiltration in templating).
- "Security by expecting people to read the docs may be a bad assumption." - Farina, helm/helm#10026 (2021)

---

## The SDK is a product, not just a CLI backend

Helm's Go SDK is consumed directly by Flux, Argo, and others; treat it as a first-class
surface with its own rules. (This is why so many "CLI" decisions are really SDK decisions.)

**Rule: The SDK returns errors; it never logs them, never panics, and never terminates the
host application.** Logging is the caller's choice.
- "Helm SDK should not be instrumented with error logs. Instead, errors ought to be
  returned. Any logging... should be left to clients." - Mungai, HIP-0021 (2024)
- "The SDK should not be terminating an application." - Mungai, helm/community#372 (2025)
- "Instead of panicking this ought to return an error." - Mungai, helm/helm#31589 (2025)

**Rule: Output sinks are injectable and default to `io.Discard`; logs go to stderr, command
output to stdout.** A service embedding Helm opts in to output.
- "writing to io.Discard by default. They set a writer if they want to capture the output." -
  Farina, helm/community#301 (2023)
- "Logs will be written to stderr by the Helm CLI client. stdout will be left for output
  from operations." - Mungai, HIP-0021 (2024)

**Rule: Prefer the Go standard library; make the logger pluggable via `slog.Handler`.**
`slog` was chosen over klog/logr precisely because it is not an external dependency.
- "slog is the preferred choice... it's part of Go standard library... [klog/logr] fell
  short because they are external dependencies." - Mungai, HIP-0021 (2024)
- Call `slog` directly rather than threading a logger object through the call graph: "I
  think we want to remove this logger that is passed around and address logging directly
  from slog." - Sirchia, helm/helm#30708 (2025)

**Rule: Minimize SDK surface area; porcelain over plumbing, batteries included.** Embedding
Helm should take ~5 lines for the common case, with primitives exposed for the advanced one.
- "we want to reduce the surface area of API... expose very high level operations, the
  porcelain... and then you have the building blocks... batteries included so you should be
  able to write like five lines and do stuff." - developer call 2024-12-20 (ASR)

**Rule: Design for SDK consumers, not just the CLI - they exercise paths the CLI never
does**, and the SDK carries the same backward-compat guarantee as the CLI.
- "The CLI doesn't create multiple transports, but someone using the SDK might." - Howe,
  helm/helm#30917 (2025)
- "exported APIs will not change on you." - KubeCon NA 2019 (ASR)

---

## How decisions get made (HIP process and governance)

**Rule: Route major features and cross-cutting decisions through a HIP; the HIP carries the
vision, the implementing PR carries the detail.** Every proposal needs a
backward-incompatibility section. The process (modeled on Python's PEP) exists to catch
design mistakes before a mega-PR.
- "we based it on the way that Python does its feature development cycle... a constructive
  dialogue." - Butcher, QCon 2020
- "Implementation details are generally addressed in the implementing PRs." - Rigby, helm/community#388 (2025)
- HIP-0001: HIPs are "the primary mechanisms for proposing major new features... and for
  documenting the design decisions." Approvals: Feature/Informational need 2 project
  maintainers; Process needs 2 org maintainers.

**Rule: Operate by lazy consensus; escalate to a vote only when consensus fails; keep the
project vendor-neutral** (no company may hold a maintainer majority).
- "there's no one company or one person or a few people who control Helm... we're there for
  the long haul." - Farina, KubeCon NA 2019 (ASR)

**Rule: Helm is critical infrastructure, so contribution rigor scales with blast radius; a
fix for one case must not break another.** Tests are a merge gate; land fixes main-first,
backport after.
- "Helm has become a critical piece of infrastructure tooling... we need to make sure a fix
  for one issue doesn't break things." - Rigby, helm/helm#12879 (2024)
- "It's... exactly how projects remain maintainable [to require] changes being made to the
  main branch first, then backported." - Rigby, helm/helm#10573 (2026)
- "some unit tests will be needed to avoid introducing corner cases." - Mungai, helm/helm#13447 (2024)

**Rule: Test all paths (negative and error, not just happy); coverage is a precondition for
refactoring; the reviewer pulls the branch, builds it, and confirms coverage is not dented.**
Every fix ships with a regression test, and a PR stays a single subject so rollback is easy.
- "we really should test the negative and test for errors... we should put test around all
  paths." - Sirchia, helm/helm#31001 (2025)
- "I am looking for test coverage for every func in this whole folder because I want to
  refactor it when helm 4 development starts." - Sirchia, helm/helm#13418 (2024)
- "a test should also be added... to prove it and to prevent a future regression." - Joe
  Julian, helm/helm#10685 (2022)

**Rule: Recognize maintainers (including non-code work) and track them in one source of
truth.** Community management is a first-class maintainer group without a repo. Sustaining
enough contributors is the project's real long-term challenge.
- "not all maintainer groups own a repository. Community management is an example... without
  owning a source repository." - HIP-0007 (2025)
- "finding enough people to help us get the work done." - Butcher, QCon 2020
- "The core maintainers are all very busy people and giving them the fewest tasks possible
  usually gets the best results... those day jobs are no longer full-time helm." - Joe
  Julian, helm/helm#11253 and #5825 (2022). The way to move a feature is to write the HIP and
  bring the PR, not to lobby.

**Rule: Design for how people do behave, not how they should; reason in explicit tradeoffs
priced in cost to thousands of downstream users.** - Farina, codeengineered.com (2018).

---

## Consistency and UX

**Rule: Apply the principle of least astonishment; reuse existing mechanisms and syntax
rather than inventing parallel ones** (SemVer range syntax, `KubeVersion`, `kubectl`
conventions, Helm-specific env vars over raw XDG).
- "I also think we should have the principle of least astonishment." - Farina, helm/helm#8332 (2020)
- "we no longer create namespaces on the fly... we wanted to follow more the pattern of
  kubectl and the Kubernetes ecosystem." - KubeCon NA 2019 (ASR)

**Rule: Think in three config surfaces for every option: flag, chart annotation, environment
variable.** - developer call 2024-12-20 (ASR).

**Rule: Cross-platform correctness is non-negotiable; Windows is first-class.**
- "we also deliver for Windows, because we know Windows is incredibly popular... we want to
  support all the developers out there." - Farina, KubeCon NA 2019 (ASR)

**Rule: Optimize for fast first success, then a path to depth ("zero to endorphins in five
minutes"); do not over-engineer validation** - state the correct pattern rather than
enumerate every wrong input. - Butcher, Kubernetes Podcast 102 (2020); Farina, helm/helm#10537 (2022).

**Rule: Follow the ecosystem's idioms in code too** (Go error strings start lowercase;
`slog` used with structured key-values, not interpolated strings; separation of concerns
between Helm types and vendored library types). - Howe, helm/helm#30603, #30774, #13480 (2024-2025).

---

## Stability and longevity

**Rule: Treat "still here and unchanged in a year" as a feature; give generous, dated
support windows so users are not rushed.** Vendor-neutral CNCF governance is part of the
stability story.
- "we have good support windows because we want to support everybody who's using Helm...
  not have people feel rushed." - Farina, KubeCon NA 2019 (ASR)
- "Helm... is a building block in the wall that holds everything together. We... needed to
  apply a little more rigor to... how we're going to change things over time." - Butcher, QCon 2020

**Rule: Predictable release cadence, because critical-infra users plan around it.** Second
Wednesday monthly (third in January, for the holiday buffer); use release candidates so
users vet before upgrading. - Farina, QCon 2020 and KubeCon NA 2025 (ASR); HIP-0002.

**Rule: Fight migration friction - work on what stops people.** A rewrite that forces every
role to relearn stalls a project; the praise Helm 3 wanted was "you just got rid of Tiller"
despite rewriting tens of thousands of lines. Users' passion for stability is real:
- "I got one death threat when we announced Helm 4... 'I will hunt you down and kill you if
  you break my charts.' And I love that passion. My wife does not." - Farina, KubeCon NA 2025 (ASR)

**Rule: Prefer maturity and the low-fi solution to novelty and hype.** - Farina,
codeengineered.com (2020).

---

## Helm 4: the sanctioned break, bounded by continuity

**Rule: A major version exists to pay down architectural debt that blocked wanted features,
and only via HIPs.** Helm 4 came at ~5-6 years because the internal architecture (not the
feature set) blocked features without breaking the public SDK.
- "We also saw where we wanted to add features but the internal architecture of Helm didn't
  provide a path forward without breaking public APIs in the SDK." - Farina, helm.sh/blog (2025)

**Rule: Even the major break is bound by continuity.** v3 charts must deploy on v4, v3
releases must be upgradable, most application-operator workflows should see no disruption; a
break that cannot be migrated "effectively becomes a different tool, which likely would
diverge the Helm ecosystem." - Jenkins, HIP-0012; continuity requirements in HIP-0012.

**Rule: Backward-incompatible chart features land behind a new chart apiVersion (v3),
additively and side by side; new runtime behavior (SSA) opts in and inherits prior choices.**
- "version v3 charts, where we can start making these changes and experimenting... it'll work
  with existing charts today, you can do it side by side." - Farina, KubeCon NA 2025 (ASR)

**Rule: A breaking major ships a supported, non-destructive migration path and a dated v3
support window** (bug fixes to 2026-07-08, security to 2026-11-11); v3 is maintenance-only,
fixes land on v4 first then backport. - Farina, helm.sh/blog (2025); Howe, helm/helm#13443 (2024).

**Rule: Helm 4 makes commands embeddable and builds reproducible** ("use helm package over
and over... get the exact same bits, the exact same digest"), and versions packaging repos
per major release (Debian practice). - KubeCon NA 2025 (ASR); Mungai, helm/helm#31671 (2026).

---


# Sources and field notes

The maintainer quotes, developer-call field notes, source list, maintainer roster, and notes on how this guide was assembled (and its limits) live in `philosophy-appendix.md`. That material is background, not review criteria.
