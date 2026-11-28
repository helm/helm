# Helm design philosophy: sources, field notes, and background

Background for the design-philosophy section of `AGENTS.md`: how it was assembled, who the maintainers are, the developer-call field notes and quotes behind the principles, the full source list, and honest limits. This is provenance, not review criteria.

## How this guide was sourced

Primary sources, actually fetched and mined:

- **Helm Developer Call recordings.** The weekly maintainer call (Thursdays, 9:30am US
  Pacific) publishes each recording to Helm's own YouTube channel (`@Helmpack`), the "Helm
  Community Meetings" playlist (`Helm Developer Call YYYYMMDD`), stated in
  `helm/community/communication.md`, which also points to the running-notes Google Doc. The
  playlist holds **380 videos** spanning 2018-2026; **377 with captions were transcribed**
  via `yt-dlp` (YouTube's timedtext API and all transcript services were token-walled; 3
  videos lack captions or were removed) and mined across 12 signal-ranked passes. Captions
  are ASR (auto-generated), so wording is approximate and speaker labels are inferred only
  where the transcript names someone. The Field notes section distills this archive.
- **Maintainer GitHub comment histories** across `helm/helm` and `helm/community` (via
  authenticated API): Farina, Butcher, Fisher, Khouzam, Dolitsky, Thomas, Hickey, Reese,
  Rigby, Mungai, Howe, Sirchia, and Julian (Julian's oldest 2019-2020 items only partially
  sampled - see Gaps).
- **All 27 Helm Improvement Proposals** (index `helm/community/hips/`).
- **The developer-call notes 2017-2021** (`helm/community/meeting-notes/*.txt`).
- **Both blog archives** (`codeengineered.com`, `helm.sh/blog`), **conference-talk
  transcripts** (KubeCon 2018-2025, the QCon "Helm: Past, Present, Future" panel), and
  **podcasts** (SE Radio 509, GCP 50, Kubernetes Podcast 102/168).


## The maintainer roster (multi-repo)

Helm is a multi-repo org; the canonical registry is `helm/community/maintainer-groups.yaml`
(established by **HIP-0007**, authored by Farina and Rigby - still draft, but the only
machine-readable source). Do not treat `helm/helm/OWNERS` as the whole project.

- **Org maintainers** (scope, vision, brand, security oversight; `MAINTAINERS.md`): Karen
  Chu (`karenhchu`), Matt Butcher (`technosophos`, chair), Matt Farina (`mattfarina`),
  Reinhard Nagele (`unguiculus`), Scott Rigby (`scottrigby`).
- **Helm core** (`helm/helm/OWNERS`): Evans Mungai (`banjoh`), George Jenkins (`gjenkins8`),
  Joe Julian (`joejulian`), Marc Khouzam (`marckhouzam`), Matt Farina, Robert Sirchia
  (`robertsirc`), Andrew Block (`sabre1041`), Scott Rigby, Matt Butcher, Terry Howe
  (`TerryHowe`). Triage: `yxxhero`, Allen Bai (`zonggen`), Ian Zink (`z4ce`).
- **Website** (`helm-www`): Jenkins, Karen Chu, Farina, Paige Calvert (`paigecalvert`),
  Rigby, Butcher, Howe, `yxxhero`.
- **Chartmuseum**: Casey Buto (`cbuto`), Josh Dolitsky (`jdolitsky`), Obinna Odirionye
  (`nerdeveloper`), Nace Sc (`scbizu`).
- **Charts group** (chart-testing, chart-releaser, actions): Reinhard Nagele, David Karlsen
  (`davidkarlsen`), Carlos Panato (`cpanato`).
- **Mapkubeapis plugin**: Farina, Sirchia, Rigby.
- **Security team** (`SECURITY.md`): Block, Jenkins, Butcher, Farina, Sirchia.
- **Emeritus** (~29, across groups): Taylor Thomas (`thomastaylor312`), Martin Hickey
  (`hickeyma`), Adam Reese (`adamreese`), Matt Fisher (`bacongobbler`), Josh Dolitsky
  (emeritus org-wide but active on Chartmuseum), Vic Iglesias (`viglesiasce`), Bridget
  Kromhout, Karena Angell (`angellk`), Michelle Dhanani (`michelleN`), Paul Czarkowski,
  Lachlan Evenson, Adnan Abdulhussein (`prydonius`), Rimantas Mocevicius, and others.

50 distinct humans total. When you cite "a maintainer," name them and their role, and
remember decisions on non-`helm/helm` repos (Chartmuseum, charts tooling) belong to those
groups.


## Field notes from the Developer Call archive (2018-2026)

Distilled from mining all 377 available Developer Call transcripts. These extend, do not
replace, the rules above; where a line restates an earlier rule it adds a vivid phrasing or
new nuance. Quotes are ASR (approximate); speakers named only where the transcript
identifies them; extra weight to Farina. Cited as `- who, dev call YYYY-MM-DD` (video id
under `youtu.be/`).

### Identity and stability
- **Boring is the goal.** "reliable boring infrastructure software that just needs to work...
  not a startup where you can move fast and break things" - Farina, 2024-05-16. Also: "if
  Helm 4's most exciting thing is we didn't break people, that's a big win."
- **Majors are rare; Helm is ancillary.** "Helm isn't a major thing, it's a minor thing in
  what they do"; aim for a major "every 2 or 3 years," and assume most users lag one or two
  majors behind - Farina, 2026-03-12.
- **Support reaches far past the official n-2**, because "the peak is Kubernetes ~18 months
  ago... most people are using unsupported versions" - Farina, 2023-03-16.
- **Open source is not open build.** "everybody uses curl, nobody gets their curl binary from
  the curl project... open source doesn't mean open build" - why Helm ships no images or
  Homebrew/Chocolatey packages - Farina, 2023-12-14.
- **Slowness is deliberate.** "we're just slower at merging things because so many people use
  it, we don't want to break people" - Farina, 2020-12-17.

### Scope
- **Not a Swiss army knife.** "Helm is not attempting to be both a floor wax and a dessert
  topping" - point users wanting full lifecycle to Argo/Flux - 2022-02-24.
- **Backward-compatible does not mean in-scope.** "make me a sandwich... or the Pizza Hut API
  - it's backwards compatible but it doesn't mean it's in scope" - 2023-08-10.
- **Helm knows its place.** "it's like the separation between apt, yum and Chef... some
  features we can say this just doesn't belong in Helm, it belongs over here" - 2023-05-04.
- **Do not route around a user's bug.** "they're trying to get Helm to ride around their bug
  which can introduce a regression... we shouldn't route around that" - Farina, 2026-01-29.
- **apt/RPM do not create the user they install for** - so namespace/service-account creation
  is out of scope - Farina, 2026-01-15. And "Argo doesn't use Helm - it takes the template
  output and does its own thing" - 2024-07-18.
- **Sprig's scope is frozen small.** "its primary user is Helm... it's not the one source of
  them all" - Farina, 2022-09-01.

### Backward and forward compatibility
- **The Python 3 cautionary tale.** "we can't have our Python 3 moment here... we're an
  enabler for an ecosystem, we can't break it, and if we do Helm will just be forked" -
  Farina, 2024-10-10.
- **An API is its response too.** "an API isn't just the arguments you pass in, it's the
  contract of what you get in response" - Farina, 2024-05-16.
- **A new warning on previously-silent behavior is itself a breaking change** - Farina,
  2025-05-22. And "we don't deprecate and break [like Kubernetes]... we're adding an extra
  level of stability on purpose" - Farina, 2023-01-12.
- **Gate compatibility on the visible thing.** Restrictions belong on the Helm version the
  user can see, not the chart version they cannot - 2026-05-14.
- **Load-bearing bugs.** "people get very emotionally attached to the load-bearing bugs"; a
  silent behavior change (recompiles, does something different) "is even worse than removing
  it" - 2021-04-01, 2022-12-08.
- **Fail loud, not silent.** A missing template function errors on old Helm (good); a
  silently-ignored new field is "an end user problem" (bad) - Farina, 2022-08-11.
- **`helm create` output is exempt** from the compatibility promise - it is a developer
  helper, not a production/automation path - 2024-04-25.

### Author vs consumer
- **Namespace creation in a chart "switches the roles/personas"** from the installer to the
  author - which is why it stays an install-time concern - Farina, 2020-02-06.
- **Maintainers rank themselves last.** "those of us who build Helm, we prioritize last after
  everybody else... to make it a significantly better experience for somebody who installs a
  chart" - Farina, 2024-11-21.
- **Put config where the knowledge lives.** Hook-parallelism safety is the chart author's
  knowledge, so it belongs in chart metadata, not a CLI flag - Farina, 2024-07-11.

### Keep the core small; dependency hygiene
- **Support burden is the primary design driver.** "we're the ones who get the support
  requests and I'm trying to avoid the support requests" - Farina, 2025-10-09. And "one small
  maintenance burden could end up being something somebody has to get out of bed at 2am for."
- **Wrap volatile dependencies behind your own API**, so a major dependency bump ships in a
  Helm minor - the "cube package" pattern that shields consumers from client-go churn -
  Farina, 2024-05-16. Do not hard-depend on a VC-funded startup's bespoke features: "a
  startup can pivot, we can't" - Farina, 2025-04-10.
- **Distrust the Kubernetes dependency.** "we import nothing from kubernetes/kubernetes except
  the client, and even there I'm iffy on trust"; Helm keeps a "shallow clone" of the client-go
  factory exposing only what it needs - Farina, 2018-07-05, 2022-05-12.
- **Only claim support for what you can test.** "we can assure it builds but we can't ensure it
  runs" - add an architecture only when it hits mainstream - 2023-08-31.

### Portability and the plugin runtime
- **One static binary, no dynamic linking.** "we produce Helm as a single static binary
  without dynamic linking so everything works everywhere" - which is why a `.so`-emitting
  engine is a non-starter and WASM is the target - Farina, 2024-11-07.
- **Embed a pure-Go WASM runtime** (wazero); reject shared-object plugins - Farina, 2025-03-06.
- **"If you build it they will come" is false.** Helm 2's pluggable engine hook (`EngineYard`)
  went unused for years and was deleted in v3 - "nobody ever came" - Farina, 2024-10-10.
- **WASM is the sandbox that lets alternative engines and post-renderers ship inside a chart**,
  so a consumer can "just grab a chart and install it" without a third-party plugin, and
  untrusted plugin code is isolated instead of "executing arbitrary executables" - Farina,
  2025-12-18; Scott/George, 2025-07-10.

### Templating and determinism
- **Alternative engines enter only through the plugin seam, never core** - a chart must "just
  work without having to look inside it" - 2023-08-10.
- **Isolate non-determinism in a "generate values" phase.** Do crypto/random/`lookup` up front
  so core rendering stays 100% reproducible; "YAML in, YAML out" as a guiding principle -
  2023-11-30.
- **Turing-complete templates make a chart's real image set unknowable** - the reason a
  declared image/BOM annotation exists - 2023-11-30.
- **Some things can't be fixed in Helm.** Template line numbers/error text live in Go's stdlib
  `text/template`; providing them would mean forking Go, so it is out of scope - Farina,
  2024-10-10.

### Values and lifecycle
- **Rollback = exactly the prior state.** "if you want to make changes then you need to roll
  forward" - adding even a label on rollback risks a non-clean rollback - 2023-11-02.
- **`null` deletes a key** - a deliberate sentinel, because the Kubernetes API rejects some
  empty/conflicting keys and users need a way to remove them - 2025-01-02.
- **Release objects need their own API versions, and release logic should be separate from
  chart logic**; upgrade logic should key off the application's own version (its DB schema
  version), not chart/packaging metadata, "because people do lots of funny things with
  packaging" - Farina, 2025-12-11.
- **Never delete by surprise.** "data loss is always bad"; honor `resource-policy: keep` even
  when set outside Helm - Joe Julian, 2025-05-01. Annotate everything Helm creates and error
  on ownership conflict (exactly one owner).
- **CRDs are cluster-global/root-level.** "think of a CRD like a feature flag on your cluster
  ... that's like a root access thing"; auto-deleting one is "deleting a production database";
  "two wrongs don't make a right" - let Kubernetes handle version changes, and Helm does not
  rewrite users' manifests - Farina, 2019-01-31, 2026-01-29, 2020-11-12, 2020-02-27.
- **Never panic.** "Helm shouldn't panic... anytime you see a panic, that's a spot to worry
  [about] a security problem"; and inconsistency is itself a bug (erroring on upgrade but
  exit-0 on install for the same condition) - 2023-05-25, 2021-02-11.

### Distribution and trust
- **Decentralization is a "free market" (Packagist model), not a hosting service.** The central
  `helm/charts` repo died of maintainer burnout and an unpayable bandwidth bill ("nobody's
  going to pick up that check") - 2018-10-25, 2020-09-24.
- **Never bake a third party's URL into the client** - own a vanity URL and redirect; a
  hard-coded external URL caused a P1 outage - 2020-08-27, 2026-04-23.
- **Content-address by archive digest** (do not assume git); use one digest-keyed cache for
  both classic repos and OCI, because name+version is neither unique nor trustworthy - Farina,
  2024-10-17. This requires deterministic `helm package` (sort files before tarring) -
  2020-05-28.
- **Do not mix OCI and repository concepts (or code).** Add OCI as a third option beside repos;
  "what value is OCI giving us if we're just re-implementing the entire chart-repository API?"
  Work within native OCI primitives, no server-side daemons - Farina, 2026-01-08, 2021-06-24.
- **Spec-compliance is not real-world compatibility.** A "technically correct" OCI-auth change
  broke real registries and "Helm became nonfunctional" - hence mandatory multi-registry
  integration testing before any major - 2024-08-01, 2026-04-23.
- **Chart signatures are location-independent** - a chart doesn't embed its repository name, so
  its signature survives moving between repos (unlike a container image) - Butcher, 2019-05-30.
- **Air-gap = a repeatable bill-of-materials.** Helm must not reach outside the air gap; it
  provides a reproducible enumeration of images, and "does not provide the trust methodology"
  because no single one exists - 2023-01-19.

### Security
- **Charts are handled as in-memory tarballs, not off disk** - "certain classes of attack
  vectors go away because you're not dealing with a filesystem"; in-memory decompression is
  bounded (zip-bomb defense) with surveyed headroom - Farina, 2024-12-12, 2025-12-18.
- **Warn, don't block, when a security-relevant behavior is already in use** ("instead of
  blocking it in the name of security we announced it"); use an env var, not a flag, for a
  footgun opt-out so it "won't spread by copy-paste" - 2024-05-23.
- **First value wins is a security stance** - a later override for the same key is an injection
  vector - 2021-04-29. And refuse an add-a-flag that reopens a hole: "fix the root cause rather
  than give people the foot gun."
- **"Working as designed" is not a CVE.** A tool printing a secret it is built to print is
  misuse, not a vulnerability - push back. And "SHA proves integrity, not authenticity;
  authenticity comes from signing" - 2024-03-14.
- **Reachability-aware scanning, never a merge gate.** Prefer `govulncheck` (call-graph aware)
  and run vuln scans as a scheduled job, never a PR gate that blocks an unrelated typo fix -
  2023-05-04. Never trust client-asserted identity or forward private credentials (the Tiller
  sig-auth lesson) - 2017-11-16.

### The SDK is a product
- **Act like a grown-up program, not a CLI.** Long-running consumers (Flux, Argo, operators)
  broke the one-shot assumption; the SDK must manage its own resources, close connections, and
  stop goroutines on context-cancel - 2025-08-28, 2021-07-29.
- **`internal/` by default; the public surface is a curated one-way door.** "if you make
  something public and that was a mistake, you're stuck" - and keeping an experiment in
  `internal/` is exactly what lets it make breaking changes safely before GA - 2025-07-24.
- **Read env/config only in the CLI and pass it into the SDK** - never read the environment
  inside a package - Farina, 2022-06-09. Don't hard-code the filesystem: expose an interface
  so a GitOps controller can back the cache with object storage.

### Governance and process
- **Skin in the game.** Org maintainers come from code maintainers - no detached managers or
  executives on top - Farina, 2018-07-19. Vendor-neutral: no auto-merge/trust privilege wired
  to one company; Apache-2 license + copyright ownership is the hard gate for adopting a
  project into the org.
- **Many small HIPs over a monolithic design doc.** Helm 3's single doc read as "howl's moving
  castle instead of the cinderella castle"; HIPs are modeled on Python's PEP process and stay
  "green" (living), and route out-of-scope PRs into a HIP but then actually move it - Butcher,
  2021-04-22.
- **Design the "why"/UX first, and evidence it.** A behavior-change HIP should "look at other
  package managers... not just say this makes sense to us" - Farina, 2021-08-12.
- **Spec-first when code, tests, and docs disagree.** For a chronically buggy subsystem (values
  coalescing) none of the three is authoritative - write down intended behavior as an
  informational HIP, then code to it - 2024-02-01, 2026-05-07.
- **Ship it and let the world test it.** "a lot of people... take it for a spin, poke holes in
  it - that's when you get a lot of testing" - Farina, 2025-10-09.
- **Lower the barrier, don't raise it.** DCO over CLA; don't auto-close contributor PRs ("less
  hostile, do it by hand"); "one false step here can be the end of the project." Bots get no
  write access (own fork + PRs only); a milestone signals commitment, so track uncommitted or
  experimental work with a label instead - 2026-01-08.
- **Best practices are descriptive** - "we build best practices around what we see people
  doing" - 2018-09-04. Test Helm's *usage* of a dependency, not the dependency itself.
- **Reach the silent ~99%.** Most users are never in the community; blog/Twitter do not reach
  them, so deprecations need non-intrusive in-tool signalling - Farina, 2020-11-05.

### AI-era contributions
- **The problem is the slop, not the tool.** "it's not just about it being generated by AI...
  it's the AI slop"; an "I used AI" checkbox solves nothing - Farina, 2026-06-11.
- **AI is allowed; a human must understand and own it, and must not be listed as co-author.**
  "folks are allowed to use AI-generated code just as they were allowed to copy-paste from
  Stack Overflow, but should not list the agent as a co-contributor" - 2026-04-30.
- **Contribution exists to mentor people - "you don't do that with AI"** - Farina, 2026-06-11.
  Require an issue before a PR (the hurdle filters slop; an issue is reason-about-able). Review
  is shifting from mechanics to scope: "does this actually belong here or not?"

### Consistency and UX
- **Do not mirror Kubernetes UX - aim to exceed it.** "Kubernetes gets so much crap for poor
  UX... the goal has been to provide a better user experience... it shouldn't be the gold
  standard" - Farina, 2025-01-09, 2025-02-06.
- **Output-stability tiers.** Adding fields to JSON/YAML output is safe; append new table
  columns to the END of the row; assume users parse by column number even though they
  shouldn't - Butcher, 2020-07-30. stdout is output, stderr is diagnostics - but re-changing it
  re-breaks users (Windows treats stderr as an error).

---


## Sources

Grouped; all fetched during research. Handles: `mattfarina` Farina, `technosophos` Butcher,
`bacongobbler` Fisher, `marckhouzam` Khouzam, `jdolitsky` Dolitsky, `thomastaylor312`
Thomas, `hickeyma` Hickey, `adamreese` Reese, `scottrigby` Rigby, `gjenkins8` Jenkins,
`sabre1041` Block, `banjoh` Mungai, `TerryHowe` Howe, `joejulian` Julian.

- **Developer Call recordings:** `@Helmpack` YouTube, "Helm Community Meetings" playlist
  (`youtube.com/playlist?list=PLVt9l4b66d5EY5Xs9OVJgvO5ss9WzrSY0`); canonical pointer in
  `helm/community/communication.md`. Transcribed via yt-dlp; cited by meeting date. Deep
  dives: `Helm Developer Call 20241220 - Helm 4 discussions Pt.1/Pt.2`.
- **Matt Farina** (weighted heaviest): codeengineered.com archive; GitHub (helm/helm #5871,
  #6184, #8332, #8453, #9791, #9903, #10026, #10077, #10537, #12653; helm/community #138,
  #175, #301, #371, #379, #394); helm.sh/blog Helm 4 posts; HIP-0007, HIP-0012 (co),
  HIP-0020; developer calls; talks below.
- **HIPs (all 27):** notably 0001 process, 0004 backward compatibility, 0006 OCI, 0007
  maintainer groups, 0011 CRDs, 0012 Helm 4 process, 0015 image/BOM annotation, 0017 OCI
  media types, 0020 Charts v3, 0021 logging, 0022 wait/kstatus, 0023 server-side apply,
  0025 resource sequencing, 0026 Wasm plugins, 0029 render-time release history. Gaps: 0013, 0028.
- **Maintainer GitHub comment histories:** Butcher/Fisher (helm/helm #1193, #1413, #1883,
  #2243, #2492, #3141, #3805, #6243, #7584, #8137); Khouzam/Dolitsky (#3557, #5242, #7345,
  #7862, #10312, #10553); Rigby (#6901, #10553, #11736, #12460, #12879, #30873, #31167,
  #31176, #31340, #31748; helm/community #235, #388); Mungai (#30697, #31250, #31589,
  #31574, #13447; community #372); Howe (#12173, #12812, #13185, #13382, #13443, #30600,
  #30993, #30917). Sirchia and Julian comment histories UNMINED (see Gaps).
- **Roster:** `helm/community/maintainer-groups.yaml`, `MAINTAINERS.md`, `SECURITY.md`,
  per-repo `OWNERS` (`helm/helm`, `helm-www`, `chartmuseum`, `helm-mapkubeapis`), HIP-0007.
- **Docs & distribution:** helm.sh/docs (topics/charts, chart_best_practices/values,
  subcharts_and_globals, version_skew, provenance, library_charts, faq/changes_since_helm2);
  HIP-0006, distributed-search archive, helm/charts README, storing-charts-in-oci blog.
- **Talks (transcripts):** QCon "Helm: Past, Present, Future" (2020, infoq.com/presentations/helm-4);
  KubeCon NA 2019 "An Introduction to Helm" (Farina/Dolitsky) and "Helm 3 Deep Dive"
  (Thomas/Hickey); NA 2018 and EU 2019 "Deep Dive: Helm"; NA 2022 "Learn About Helm And Its
  Ecosystem"; NA 2025 "Introducing Helm 4" (Farina/Sirchia). Talk quotes are ASR without
  timestamps unless from the InfoQ text transcript.
- **Podcasts:** SE Radio 509, GCP 50, Kubernetes Podcast 102/168.


## Honest gaps (do not paper over)

- **The full Developer Call archive (377 of 380 videos) is now transcribed and mined**
  (see Field notes), but captions are **ASR**: wording is approximate and most speakers are
  unlabeled (attributed only where the transcript self-identifies, so many strong statements
  are "unattributed"). Verify a quote against the video before citing it as verbatim. The
  post-2021 running-notes Google Doc remains unreachable.
- **Conference-talk quotes have no per-line timestamps** (timed YouTube endpoints were
  token-walled); they carry Ctrl-F "locate-by" anchors instead.
- **`Learning Helm`** (O'Reilly; Butcher, Farina, Dolitsky) is the deepest single source
  and was not web-accessible.
- Some blog/chart lines are close paraphrase from fetch summaries; verify before quoting as
  verbatim. Podcast and developer-call quotes are auto-transcripts.


## Known gaps and do-not-overclaim

- **Julian's oldest 2019-2020 comments only partially sampled** (2020-2026 saturated);
  Sirchia and Julian are otherwise mined. HIP-0025's "Joe" is **Joe Beck (`joebeck5705`)**,
  not Joe Julian - do not attribute HIP-0025 to Julian. Julian's CRD *conversion-webhook* and
  lifecycle reasoning appears in both the 2021 dev-call notes and helm/community#379.
- **Full Developer Call archive mined** (377/380 videos, see Field notes), but ASR wording is
  approximate and most speakers unlabeled (attributed only where the transcript
  self-identifies). Post-2021 running-notes Google Doc unreachable.
- **Talk quotes lack per-line timestamps** (timed YouTube endpoints token-walled); they have
  Ctrl-F locate-by anchors in the working files, not shown here.
- **`Learning Helm`** book text not accessible. **`Learning Helm`** and PR inline-review
  threads remain the two richest unmined veins.
- Tiller-removal quotes are Fisher's/Butcher's, not Farina's. HIP-0004 was authored by
  Khouzam and Butcher. No enumerated non-goals doc exists; no formal "Why Go templates?"
  FAQ - present the engine as defended-and-retained (v4 reconsidered then kept it, adding
  YAMLScript as a mixable in-chart option), not permanently closed. Attribute precisely.

