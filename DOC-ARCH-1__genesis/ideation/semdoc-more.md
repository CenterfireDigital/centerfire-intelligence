Now, along with all of this, we're going to be creating a multi-faceted application that is fully intended to be be AI maintained. I know how we structure directories for human teams, but is there a better way to do it for AI code creation and maintanence. For example, this app I'm building will use multiple languages (Rust, Go, Python, Node/React). Do we organize code by language, by purpose? Is there a way that would be better with the SemDoc set up we're trying to acheive? Sure, humans might have to look at it and maybe later we can do some sort of symlink based human struture to make it easier, but for pure AI ease and speed of use, especially with SemDoc, what do you think?
ChatGPT said:
Thought for 7s

Love this question. Short answer: organize by “Capability → Interface → Implementation (per language)”, not by language or team. Make the capability (what the software does) the primary key of the filesystem, and let languages be “leaves.” This lines up perfectly with SemDoc (capabilities, contracts, effects) and makes AI routing/packing trivial.

Below is a pragmatic blueprint you can adopt today.

Why capability-first?

Stable semantics: Capabilities (auth, billing, ingest, ranking, rendering, etc.) change far more slowly than file layouts or language choices. Stable anchors = better retrieval.

AI packing: Given a change to “CAP-RANK-002,” the assistant can load the capability folder, find language implementations, contracts, tests, flags, telemetry—all co-located.

Cross-lang cohesion: Rust lib + Go worker + Python model + Node/React UI for the same capability sit under one roof; SemBlocks and contracts bind them.

Recommended layout (monorepo; capability-first)

Top-level: repo control plane + shared infra; then capabilities.

/sem-doc/                         # repo control plane (SemDoc v0.2+)
/platform/                        # shared infra (tooling, CI glue, schemas, SDKs)
  build/                          # Bazel/Pants/Nx/Turbo configs
  schemas/                        # OpenAPI/AsyncAPI, JSON Schemas used across caps
  libs/
    js/                           # shared JS libs (ESM)
    go/
    rust/
    py/

/capabilities/
  CAP-AUTH-001/                   # 1 folder per capability (ID is canonical)
    semdoc.yaml                   # scoped SemDoc (overrides/extends repo semdoc)
    contracts/                    # machine contracts surfaced per interface
      http.openapi.yaml
      events.asyncapi.yaml
      data.schema.sql
    domain/                       # domain models (language-neutral)
      glossary.md
      ontology.jsonld             # local graph extensions
    interfaces/                   # narrow ports the world sees
      http/                       # OpenAPI slices for just this capability
      events/
      cli/
    impl/                         # implementations grouped by role, *then* language
      service/                    # long-running services (APIs/workers)
        go/                       # Go service for auth
          cmd/api/
          internal/...
        rust/                     # Rust token service
        node/                     # Node microservice if needed
        py/                       # Python worker / feature
      lib/                        # language libraries for this capability
        js/
        go/
        rust/
        py/
      ui/                         # UI slices bound to this capability
        web-react/
          src/
        admin-react/
    tests/                        # per-capability test batteries
      unit/
      property/
      integration/
      e2e/
    telemetry/
      events.yaml                 # canonical events & props
      dashboards/                 # Grafana/Looker configs
    flags/
      flags.yaml                  # all flags touching this capability
    runbooks/
      oncall.md
      rollout.md
    maps/
      code-map.json               # symbols→purpose→tests→graph_id
      test-map.json
    data/
      fixtures/
      migrations/
    README.md                     # human overview (short; links into contracts)

/products/                        # product compositions (if many apps share caps)
  app-foo/
    composition.yaml              # which capabilities + versions + feature flags
    deployments/                  # env-specific topology & releases

Why this shape?

A capability is a cohesive unit of meaning. Inside it:

interfaces/ expose its contract to the world,

impl/ houses polyglot implementations grouped by role (service/lib/ui),

contracts/ & maps/ give the AI everything to edit safely,

tests/ live beside what they validate (and match SemBlocks tests.oracles),

telemetry/ & flags/ encode runtime guardrails.

Language placement rules (AI-friendly)

Language is never top-level. It’s a property of the implementation, not the organizing principle.

Inside impl/:

Role first (service/, lib/, ui/). The AI picks role from Change Intent, then drops to language.

Keep one exported symbol per file for JS/TS; small modules for Go/Rust/Python.

File names should embed the graph_id tail (stable lookup):
buildPersonaContext.lexi.Function.buildPersonaContext.ts

SemBlocks live directly above each exported symbol; file-level SemBlock at the top of each module.

Graph & vector fast paths

Every CAP-XXXX/ folder includes:

/maps/code-map.json (generated from SemBlocks)

/contracts/*.yaml (OpenAPI/AsyncAPI/data schemas)

/telemetry/events.yaml

During packing, the assistant:

loads the capability folder from composition.yaml or Change Intent,

pulls SemBlocks for target symbols + one-hop neighbors,

fetches oracles in /tests/**,

respects /flags/flags.yaml and /telemetry/events.yaml.

Products vs. capabilities

For multi-app portfolios, products are compositions of capabilities. They don’t own the code; they reference versions/flags:

Good: clean reuse; product switching is just changing the composition.

Legal: easy to export a product: copy its composition + included capability versions (and their PSM adapters).

Human-friendly overlays (optional)

If you want a familiar dev surface: create a symlinked or generated “human view”:

/human/
  by-language/
    js/ -> ../../capabilities/**/impl/**/js
    go/ -> ../../capabilities/**/impl/**/go
  by-service/
    generator/ -> ../../capabilities/CAP-GEN-001/impl/service/*


A small CLI can generate this per developer preference without changing the canonical, AI-optimized layout.

What about monorepo vs. polyrepo?

Monorepo (recommended) for AI maintenance:

single graph across capabilities,

consistent tooling (Bazel/Pants/Nx/Turborepo),

unified vector/graph indices.

Polyrepo only when legal/tenancy demands isolation:

Keep the same internal layout under /capabilities.

Registry of capabilities lives in an org-level catalog; products still compose via composition.yaml.

Naming & IDs (make AI navigation deterministic)

Capability IDs: CAP-<DOMAIN>-<NNN> (e.g., CAP-AUTH-001).

Graph IDs: lexi:<Kind>/<Capability>/<Name> (e.g., lexi:Function/CAP-AUTH-001/issueToken).

File headers include graph_id; filenames include the Name for grep-ability.

Commit trailers (SemCommit) must list Graph-IDs touched.

Tests & datasets

Keep tests inside the capability to avoid scatter:

tests/unit/ mirror impl/**/ trees,

tests/property/ derive from contract.post expressions,

tests/integration/ mount interfaces declared in contracts/.

Fixtures belong in capability/data/fixtures/ and are referenced by SemBlocks.

Feature flags & config

All flags affecting the capability live in flags/flags.yaml with:

default, owners, blast radius note, kill-switch behavior.

Config keys (with types/ranges) in contracts/config.yaml so assistants don’t hardcode.

Build & CI

One build graph at /platform/build/:

Bazel/Pants/Nx/Turborepo can address paths; targets live under capability folders.

CI validates:

SemBlocks/Contracts schema,

coverage of exported symbols,

effects drift vs. contract,

perf budgets (microbench subsets),

telemetry schema consistency.

Good / Bad / Ugly of this layout

Good

AI can do locality-aware edits: everything relevant to a capability sits together.

Cross-language work (e.g., changing Rust core + Node API + React UI) stays scoped to one folder.

Legal separation by capability is straightforward (copy/ship a folder + SemDocs + PSM adapter).

Bad

Humans used to language-first repos may need an overlay (symlinks/IDE workspace presets).

Some tooling defaults (e.g., Go module roots) need minor config to work under nested paths.

Ugly

If a capability explodes in size, the folder can become heavy. Mitigate by sub-capabilities (e.g., CAP-AUTH-001-session/, CAP-AUTH-002-keys/) with clear dependencies in the graph.

Conventions that supercharge the AI loop

Exported symbol ⇄ test oracle: 1:1 or small set; name tests Symbol_contract_*.

One responsibility per file (esp. TS): makes contract diffs precise.

Explicit resource names in effects (db:users, event:auth.issued).

Telemetry co-location: declare in telemetry/events.yaml, assert in tests.

SemCommit trailers: always list Change-Intent, Graph-IDs, Effects, Risk.

If you want, I can sketch a tiny catalog (/sem-doc/catalog.yaml) that enumerates capabilities, owners, and maturity, plus a product composer (/products/*/composition.yaml) example so your orchestrator can build deployables per product automatically.

You said:
Also, keep in mind, that this whole thing is verly likely to have AI agent orchestration running inside of it. So, let's evaluate again, take that into consideration and think about this as a blueprint for other projects. This doesn't have to be ready to submit to a standards body or anything, for now it's just local use but really needs to be done correctly. Take a longer thinks and evaluate again.
ChatGPT said:
Thought for 15s

got it—let’s re-evaluate with agent orchestration inside the app as a first-class citizen and turn everything we’ve sketched (SemDoc, SemBlocks, per-repo PSMs, semantic commits, capability-first layout, AI-native SDLC) into a coherent, reuseable blueprint you can apply across projects.

Below is the “do-it-right” version for local adoption (not standards-body formal, but crisp enough to implement).

1) Core stance

AI-maintained systems require AI-maintainable structure. Organize the repo, metadata, and runtime around capabilities, contracts, and agents, not around teams or languages.

Two brains, one canon:

GA (general assistant) for broad reasoning.

PSM (per-repo specialist) for house style & constraints.

SemDoc is the source of truth both must obey.

Agents are just specialized editors/executors bound by contracts and tools. They plan, act, and verify against SemDoc.

2) Repo blueprint (AI-first, capability-first, agent-aware)
/sem-doc/                         # control plane (schemas, catalog, policies)
  semdoc.yaml                     # repo SREADME (indices, owners, tenancy)
  catalog.yaml                    # list of capabilities & maturity
  domain.graph.jsonld             # ontology for code + agents + tools
  policies/                       # org rules (security, PII, tenancy)
  packs/                          # reusable contract packs & prompts

/platform/                        # shared infra/tooling
  build/                          # Bazel/Pants/Nx/Turbo configs
  schemas/                        # OpenAPI, AsyncAPI, JSON Schemas
  libs/{js,go,rust,py}/           # shared libraries/adapters
  agents-sdk/                     # unified agent & tool interfaces (thin)

/agents/                          # agent runtime & orchestration
  orchestrator/                   # router, planner, scheduler, watchdog
  agents/                         # concrete agents (bound to capabilities)
    code-refactor/                # example: refactor agent
      AgentSpec.yaml              # scope, tools, guardrails, SemBlocks
      prompts/                    # system & behavior prompts
      tools.list.yaml             # tool contracts this agent can call
      memory/                     # memory config (episodic/semantic/graph)
      tests/                      # agent self-tests & red-team scripts
    qa-runner/
    perf-guardian/
    release-manager/
  tools/                          # tool adapters (actions with contracts)
    git/ToolSpec.yaml
    tests/ToolSpec.yaml
    db/ToolSpec.yaml
    feature-flags/ToolSpec.yaml
  policies/                       # agent-level policies (rate limits, authZ)
  telemetry/                      # traces schemas for agents (OTel)

/capabilities/
  CAP-XXXX-###/
    semdoc.yaml
    contracts/                    # OpenAPI/AsyncAPI/data & config
    domain/                       # glossary + capability-level ontology
    interfaces/{http,events,cli}/
    impl/                         # role-first, then language
      service/{go,rust,node,py}/
      lib/{go,rust,js,py}/
      ui/{web-react,admin-react}/
    tests/{unit,property,integration,e2e}/
    telemetry/{events.yaml,dashboards/}
    flags/flags.yaml
    maps/{code-map.json,test-map.json}
    runbooks/{oncall.md,rollout.md}

/products/                        # product compositions of capabilities
  app-foo/
    composition.yaml              # caps + versions + flags
    deployments/                  # env topologies & releases


Humans who prefer language-first views can use a generated /human/ overlay (symlinks or IDE workspace presets). The canonical layout remains capability-first.

3) SemDoc extensions for agents (just enough to be powerful)
3.1 AgentSpec (what an agent is allowed to do)

Lives in /agents/agents/<name>/AgentSpec.yaml

version: "0.1"
id: "agent.code-refactor"
purpose: "Safe refactors within declared contracts"
scope:
  capabilities: ["CAP-AUTH-001","CAP-GEN-001"]     # optional narrowing
  graph_ids: ["lexi:Function/*"]                   # allowed kinds
guardrails:
  max_diff_lines: 400
  requires_semchangeplan: true
  forbidden:
    - "contract.stage == 'deprecated' -> no new code"
    - "security.pii == 'sensitive' -> no direct edit; propose plan"
tools:
  - id: "tool.git"
  - id: "tool.tests"
  - id: "tool.flags"
  - id: "tool.search"
memory:
  vector_collections: ["semblocks","adrs","tests","failures"]
  graph_entrypoint: "neo4j+s://..."
packing_rules:
  - "symbol.semblock"
  - "file.semblock"
  - "neighbors.one_hop"
  - "tests.oracles"
  - "adr.related"
review:
  auto: "risk == 'low' && tests.pass && perf.within_budget"
  human: "risk != 'low' || contract.changed"
telemetry:
  traces: "otel"       # agent spans, tool spans, model spans
  events:
    - "agent.plan.created"
    - "agent.diff.proposed"
    - "agent.eval.passed"
tenancy: "isolated"    # or shared

3.2 ToolSpec (the only way an agent acts)

Lives in /agents/tools/<name>/ToolSpec.yaml

version: "0.1"
id: "tool.tests"
summary: "Select & run impact tests"
inputs_schema: "jsonschema://tests.run.request"
outputs_schema: "jsonschema://tests.run.result"
effects:
  reads: ["tests:*","fixtures:*"]
  writes: ["report:ci"]
security:
  authn: "serviceToken"
  authz: ["role:ci"]
contracts:
  pre: ["request.test_list.len > 0"]
  post: ["result.passed.len + result.failed.len == request.test_list.len"]
costs:
  time_budget_ms: 120000
  rate_limit_per_min: 5


Both AgentSpec and ToolSpec are SemDoc-native: they use the same contract vocabulary (pre/post/effects/security). That lets you reason about agents like code.

4) Orchestrator design (inside your app)

Responsibilities

Routing: choose GA vs PSM vs hybrid vs specific agent.

Planning: convert a Change Intent into a SemChangePlan; map to tools.

Scheduling: enforce budgets, rate limits, and concurrency for agents/tools.

Execution: drive Plan→Act→Check loop (tests, perf, telemetry).

Safety: kill switches, timeouts, rollback orchestration (feature flags, revert PRs).

Tracing: OpenTelemetry spans for every step; attach SemCommit metadata.

Data plane

Context fabric: SemBlocks + vector + graph (as already defined).

Experience DB: episodes for agents (prompts→actions→outcomes) to refine PSMs and policy.

Control plane

Policies: tenancy, security, PII, model registry, allowed tools, max diff sizes.

Registries: Agent registry; Tool registry; Model/Adapter registry (PSM versions).

Evaluators: unit/property tests, red-team suites, perf smoke, telemetry assertions.

5) AI-native SDLC with agents (concise lifecycle)
Change Intent → (orchestrator)
   → plan (SemChangePlan) [GA/PSM]
   → guarded edit (agent) using tools
   → impact tests + property tests + perf checks
   → SemCommit written with structured trailers
   → auto or human review based on risk
   → merge + rollout (flags, canaries)
   → runtime monitors validate invariants
   → episode logged for learning


This replaces sprints/boards with a state machine and policy gates. It scales from tiny to large teams.

6) Memory model for agents (keeps them sharp, not leaky)

Episodic (short-term): current CI, plan, diffs, local test outcomes.

Semantic (medium-term): SemBlocks, ADR summaries, patterns (vector DB).

Structural (long-term): Code & capability graph (Neo4j).

Experience replay: outcomes of past runs, linked to commits and tests (Postgres/ClickHouse).

Tenancy fences: per repo/capability; portfolio adapters consume only shared episodes.

7) Observability & audit (non-negotiable)

OpenTelemetry end-to-end: request → agent plan → each tool call → tests → merge.

Semantic commits: structured footers (Change-Intent, Graph-IDs, Effects, Risk, Tenancy, Telemetry, Perf).

Lineage views: “This production change came from CI-2025-00123 → plan v3 → code-refactor agent → tests suite X → merged by policy Y.”

Red-team & incident loops: agents have self-tests, and incidents feed new negative cases.

8) Security model (least privilege for agents)

Tool-level authZ: agents can only call whitelisted tools; tools own credentials, not agents.

Read vs write contexts: distinct identities; planning reads broad context, writing requires elevated short-lived tokens.

Secrets isolation: tools fetch secrets via broker; never pass them through the LLM context.

Prompt-injection hardening:

Restricted context packing (SemBlocks + contracts only).

No raw untrusted text directly to write-capable agents.

Contracts validated post-diff (CI gate).

9) Testing strategy (for both code & agents)

Property tests from contracts: auto-generate quickchecks from contract.post expressions.

Impact test selection: graph-aware; run the smallest sufficient set fast, then a wider ring.

Perf smoke: microbench for any function with performance budgets.

Agent red-team: prompt-injection, jailbreaks, tool-abuse scenarios; must fail closed.

Sandboxed execution: ephemeral container/VM for risky actions; rollbacks are rehearsed (feature flag off, revert PR).

10) Model strategy (keep nimble)

PSM adapters per repo/capability using LoRA/QLoRA; load dynamically.

Portfolio adapter for shared patterns only (opt-in via tenancy flags).

Router rules: tiny PSM for idioms & refactors; GA for novel reasoning; hybrid for complex changes.

Version pinning & A/B: adapters are versioned; compare against a baseline in shadow mode before promotion.

11) Governance & legal (future-proof)

Tenancy everywhere: isolated|shared on repos, capabilities, agents, tools, and episodes.

Exportability: selling a product = ship its capabilities + SemDoc + PSM adapters + episodes; portfolio artifacts exclude isolated.

Compliance hooks: SemDoc policies/ enumerate residency, PII classes, retention; CI enforces.

12) Anti-patterns to avoid

Language-first repo roots. (AI packing suffers; cross-lang edits fracture.)

Agent with blanket tool access. (Use ToolSpec + policies; least privilege.)

Unbounded context stuffing. (Use packing rules; one-hop graph by default.)

“Docs as prose only.” (Everything important must be machine-checkable contracts.)

Sprints/boards as control points. (Use the state machine + policies, not meetings.)

13) Rollout plan (safe & incremental)

Phase 0 (1–2 days)

Adopt capability-first layout for one capability.

Add SemBlocks & Contracts to 10–20 key symbols.

Turn on SemCommit trailers in PR template.

Phase 1 (1–2 weeks)

Introduce orchestrator (router only).

Add two agents: code-refactor and qa-runner, both read-only or guarded.

Start logging episodes & traces.

Phase 2 (2–4 weeks)

Enable guarded writes (small diffs) with policy gates.

Train first PSM adapter from your episodes.

Add perf-guardian & release-manager agents.

Replace backlog grooming with Change Intents queues.

Phase 3 (ongoing)

Expand contracts coverage and property tests.

Add portfolio adapter for shared patterns only.

Tighten security & red-team suites.

Generate the human overlay for developer comfort.

14) What “good” looks like (definition of done)

80%+ of exported symbols under SemBlocks with contracts.

Agents operate under ToolSpec with least privilege; all actions traced.

Change Intents flow through plan→act→check automatically; risky edits require human review.

P95 perf and telemetry budgets enforced by CI; flags are first-class.

Semantic commits present on 95%+ merges; vector/graph show clean lineage.

Tenancy fences demonstrably prevent cross-product knowledge leakage.

If you want, I can convert this into a short “Blueprint README” (one page you can drop into /sem-doc/) and a checklist you can track in your first repo. When you’re ready, we can also sketch AgentSpec/ToolSpec minimal JSON Schemas so validators and CI gates are straightforward.

You said:
Regarding #6, short/medium/long term we've already got Neo4j in the dev environment ready to use. WE're doing rapid AI development, Iwe ahve to stop thinking in terms of human phases, dev team timeles, etc. So, once again. Start over, think about what we're doing and make a detailed plan I can stick in an architectural envrionment. FYI - The emulator/vm stuff to interact with Claude Code mentioned way above is actually going to be done in a combination of Go/Rust/Python and Node. Well for the whole symantic intelligence project. This is all AI first thinking. So let's do it.
ChatGPT said:
Thought for 26s

Alright—fresh pass with AI-first, agent-inside thinking and Neo4j already online. Below is a drop-in blueprint you can paste into your architecture docs. It’s opinionated, capability-first, and designed for continuous operation (not phases or ceremonies).

AI-Maintained System Blueprint (v1)
0) Objectives

Autonomous upkeep: Agents plan→edit→test→ship within strict contracts.

Single source of truth: SemDoc (repo control plane + inline SemBlocks) governs code, agents, tools, telemetry, and policies.

Graph-centric brain: Neo4j is the live map of code, capabilities, agents, tests, data, flags, and risk.

Continuous flow: No sprints. Work is Change Intents and state transitions with policy gates.

Portable IP: Per-capability isolation, tenancy flags, and exportability (PSM adapters + episodes).

1) System Context (at a glance)

General Assistant (GA): broad reasoning (external or internal model).

PSM(s): per-repo/per-capability small adapters specialized to your codebase.

Orchestrator: routes requests, builds plans, schedules agents, enforces policy, logs episodes.

Agents: constrained executors (code-refactor, QA-runner, perf-guardian, release-manager, etc.).

Tools: privileged actions with explicit contracts (git, tests, flags, DB, search, microVM).

Context Fabric: vector index (semantics), Neo4j (structure), SemDoc files (contracts).

Observability: OpenTelemetry traces across plans, tools, tests, merges, rollouts.

2) Canonical Repo Layout (capability-first, agent-aware)

/sem-doc/ – repo control plane (SREADME, catalog, policies, graph/pack schemas).

/platform/ – shared build config, schemas, SDKs, adapters.

/agents/ – orchestrator, agent specs, tool specs, prompts, policies, telemetry.

/capabilities/CAP-XXXX-###/ – each capability bundles interfaces, implementations (service/lib/ui by role, then language), tests, telemetry, flags, maps, runbooks.

/products/<app>/ – compositions of capabilities + deployment topology.

Rationale: AI and humans land on the capability directory and find everything—contracts, code, tests, telemetry, flags—in one place. Languages are leaves, not roots.

3) Roles by Language (why each exists)

Rust: microVM/sandbox supervisor, resource isolation, low-level file ops, safety enforcers.

Go: orchestrator runtime, tool daemons (git, tests, flags), high-throughput schedulers, gRPC servers.

Python: model adapters (LoRA/QLoRA), embedding pipelines, evaluation harnesses, data workers.

Node/React: IDE & CLI gateway, developer UX, API façades to GA/PSM, admin UI for policies and dashboards.

Everything speaks gRPC or HTTP+JSON with explicit ToolSpecs; no ad-hoc shelling from agents.

4) Neo4j as the Source of Structure

Core labels (nodes):

Capability, Module, Function, Endpoint, Event, Table, Dataset, Flag

Test, ADR, Contract, ChangeIntent, Commit, Agent, Tool, Episode, ModelAdapter

Key relationships:

(:Capability)-[:CONTAINS]->(:Module|:Endpoint|:Event|:Flag|:Contract)

(:Module)-[:DECLARES]->(:Function)

(:Function)-[:TESTED_BY]->(:Test)

(:Function)-[:WRITES|READS]->(:Event|:Table|:Dataset)

(:Function|:Endpoint)-[:GOVERNED_BY]->(:Contract)

(:ChangeIntent)-[:TARGETS]->(:Function|:Module|:Endpoint|:Capability)

(:Commit)-[:TOUCHED]->(:Function|:Contract|:Flag|:Event)

(:Commit)-[:IMPLEMENTS]->(:ChangeIntent)

(:Agent)-[:CAN_USE]->(:Tool)

(:Episode)-[:FROM]->(:Agent)

(:ModelAdapter)-[:SPECIALIZES]->(:Capability|:Repo)

Why: lets the orchestrator compute blast radius, pick tests, enforce reviewers, and generate SemChangePlans with one-hop neighborhoods.

5) Vector Index (semantics that matter)

Collections (with tight metadata):

semblocks (inline contracts, invariants, effects; chunk = one symbol)

adrs (one ADR per chunk; link to affected graph_ids)

tests (oracle summaries and reasons-to-exist)

telemetry (event/metric schemas)

failures (postmortems, flaky test profiles)

commits (semantic footer digest + diff summary)

Embedding rules: prefer small, atomic units tied to a stable graph_id, include {repo, capability, file, symbol, commit, stage, tenancy}.

6) Orchestrator (always-on loops)

Core loops (continuous; no phases):

Intent Intake: ingest Change Intents from UIs, telemetry anomalies, GA proposals. Normalize to capability + graph targets.

Planning: for each Intent, build a SemChangePlan using SemBlocks + graph neighbors + similar Episodes.

Execution: schedule agents with ToolSpecs to produce diffs, respecting contracts and budgets.

Verification: run impact tests, property tests (from postconditions), perf smokes, telemetry schema checks.

Governance: write SemCommit trailers, enforce risk/tenancy policy, request human review if required.

Release: flip flags/canary deploy; monitor runtime invariants.

Learning: log Episode (prompt→actions→outcomes), refresh PSM adapters on a cadence.

Schedulers: Go services with per-capability queues and SLAs; backpressure is applied when CI/test capacity tightens.

7) Agents & Tools (least privilege by contract)

Agent examples:

code-refactor: small diffs under strict contracts; cannot change security/PII unless plan approved.

qa-runner: chooses tests by graph impact; records flaky profiles; can retrigger.

perf-guardian: microbench runners and perf budgets; blocks merges on regression.

release-manager: flag orchestration, canaries, rollback policies.

semdoc-warden: validates SemBlocks/Contracts coverage and freshness; files Debt Intents.

Tool examples (each with ToolSpec):

git (read/branch/diff/write within a capability scope)

tests (select/run/report with time budgets)

flags (read/write with ownership and blast radius)

db (schema diff/validate; no prod data access)

microvm (spawn sandbox for build/run; resource caps)

search (vector + graph query helpers)

telemetry (emit/check event schemas)

Policy: agents do not own secrets; tools do. Agents call tools; tools enforce authZ against policies.

8) Emulator/VM Integration (Claude Code & friends)

MicroVM Supervisor (Rust): spins ephemeral sandboxes for build/test/run; enforces CPU/mem/PIDs/FS isolation.

Tool Adapters (Go): “microvm” tool exposes Start/Exec/Collect with time and resource budgets; produces artifacts (logs, coverage, perf profiles).

Editor Gateway (Node): bridges IDE/CLI (including Claude Code) to orchestrator; streams SemBlocks + plan context; returns diffs and validation results.

Python Workers: prepare embeddings, evaluate adapters, generate property tests from contract postconditions.

Contract: no write-capable agent runs outside a microVM; diffs leave the sandbox only after verification.

9) SemDoc Contracts (enforcement backbone)

Inline SemBlocks above every exported symbol: pre, post, invariants, effects (reads/writes/calls), security/PII, perf budgets, feature flags, tests oracles, telemetry events, compat, tenancy.

Module/capability contracts summarize interfaces, datasets, SLOs, owners, rollout/rollback.

Validation gates: signature drift, effects drift, perf drift, telemetry drift, security drift → blocked unless the contract is updated and risk accepted.

10) Change Intents & Semantic Commits (flow control)

Change Intent: a small, structured declaration of what to change and under which constraints; bound to capabilities and graph targets.

SemChangePlan: auto-generated plan citing contracts/invariants/tests/flags it touches.

Semantic Commit Trailers: machine footers on merge (Intent ID, Graph-IDs, Effects delta, Risk, Perf, Security, Tenancy, Tests, Telemetry, ADR link). CI cross-checks footers against the diff and contracts.

11) Security, Tenancy, Compliance

Tenancy: isolated|shared at repo, capability, agent, tool, and episode levels; CI enforces fences; portfolio adapters train only on shared.

Least-privilege: tool-scoped tokens; short-lived write tokens via orchestrator.

Secrets: fetched only inside tools; never injected into LLM contexts.

PII classes: declared in contracts; telemetry/log schemas validated.

Exportability: to sell a product, ship its capability folders, SemDoc, adapters, and Episodes; portfolio artifacts exclude isolated by policy.

12) Observability & Audit

Traces: every plan/tool/test/merge/rollout step emits OTel spans tagged with {capability, graph_id, intent, commit}.

Events: agent.plan.created, agent.diff.proposed, tests.impact.done, perf.budget.met|fail, rollout.canary.ok|fail.

Dashboards: capability SLOs, debt heatmaps (contract coverage, drift), agent pass rates, merge cycle time, rollback rates, P95 deltas per module.

Forensics: lineage graph: Intent → Plan → Agent/Tool actions → Tests → Commit → Rollout → Runtime conformity.

13) Model Strategy (GA + PSM)

Router rules: GA for novel reasoning/planning; PSM for idiomatic edits/refactors/tests; hybrid for complex changes (GA-plan, PSM-implement, GA-review).

Adapters: LoRA/QLoRA per repo/capability; stored with version metadata and tenancy.

Training sources: Episodes, SemBlocks, tests, semantic commits; strict secret/PII scrubbing.

Evaluation: offline golden sets + online shadow runs; promotion when passing gates.

14) Continuous Bring-Up (no phases—always-on readiness checks)

These run in parallel from day one; each has a readiness “gate”:

Graph hydration: extract SemBlocks + interfaces → Neo4j edges; gate: ≥ N% exported symbols mapped.

Vector hydration: embed SemBlocks/ADRs/tests; gate: basic retrieval returns correct contexts for K sampled symbols.

Contracts validator: gate: contracts validate on commit; signature/effects drift caught.

Agent sandbox: gate: code-refactor agent can produce a trivial, verified diff inside microVM and exit cleanly.

Impact tests: gate: graph-selected tests run for sample diffs within time budget.

Perf guard: gate: microbench harness runs for at least one capability with stable baselines.

Semantic commits: gate: merge blocker if footers missing/inconsistent.

Tenancy fences: gate: attempts to mix isolated into portfolio adapter are rejected and logged.

Observability loop: gate: full trace from Intent→Commit visible in dashboard for sample change.

Treat each gate as a service SLO. If a gate regresses, the orchestrator throttles write-agents until green.

15) Governance: policies as data

Risk policy: auto-merge only when risk=low, tests pass, perf within budget, no contract change; else human review.

Diff size policy: cap lines changed; larger diffs require explicit human override.

Edit scopes: forbid agents from touching contract.stage=deprecated or security.pii=sensitive without plan approval.

Rollout policy: canary % and kill-switches by capability criticality.

All policies live in /sem-doc/policies/ and are referenced by the orchestrator at runtime.

16) Success KPIs (watch these)

First-pass test pass-rate for agent diffs (target ≥ 80% then ratchet).

Median time-to-green per capability (minutes, not days).

Contract coverage of exported symbols (target ≥ 80%, then grow).

Contract drift rate (should trend down as PSM learns).

Rollback rate (target near zero on low-risk auto merges).

Perf budget adherence (p95 deltas near zero on non-perf CIs).

Debt heatmap burn-down (coverage ↑, stale contracts ↓).

17) What to implement first (concurrent, not sequential)

Wire Neo4j to SemBlocks + interfaces and keep it fresh on every commit.

Stand up the orchestrator (Go) with read-only routes to GA/PSM and graph/vector.

Add the microVM tool (Rust supervisor + Go adapter) and run tests inside it.

Enable SemCommit trailers in your merge policy.

Create two agents: code-refactor (small diffs only) and qa-runner.

Turn on contract validation as a merge gate for a single capability, then expand.

Everything else scales out from these primitives.

18) Anti-drift safeties (hard stops)

No write agent runs without a SemChangePlan citing contracts and tests.

No merge without a semantic commit that cross-checks with the diff.

No adapter promotion without offline eval + online shadow passing.

Any perf/telemetry budget breach blocks merge unless policy override.

Tenancy violations hard-fail and alert.
