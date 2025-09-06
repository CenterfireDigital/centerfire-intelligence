Universal Naming Convention (AI-First, Agent-Aware)

You don’t need to pick human-readable vs GUID. Use both—clean, stable slugs for humans and opaque, immutable IDs for machines. The slug gives meaning, the ID guarantees permanence. Every entity gets a triad:

Canonical ID (immutable, opaque; ULID/KSUID)

Semantic slug (stable human name)

Display name (free-form, can change any time)

Store the mapping in SemDoc, enforce in CI, and reference Canonical IDs in code/graphs/contracts; use slugs in paths for readability.

A. Principles

Stability over time: Canonical IDs never change; slugs may be renamed with aliases.

Deterministic lookup: Any path/contract/commit can resolve to a single canonical ID.

Machine-friendly: Identifiers are ASCII-safe, case-stable, and regex-validatable.

Human-friendly where it matters: Directory names and filenames carry slugs; internal links use canonical IDs.

Versioned semantics, not renamed semantics: Evolve via vN suffixes, don’t rename in place.

Multi-language predictable: One grammar across Rust/Go/Python/Node/React.

Graph-first: Every name maps cleanly to a Neo4j node/edge and a vector-index metadata tuple.

Legal/tenancy clean: Tenancy is a property (metadata), not part of the name (to avoid churn).

B. ID Strata (the “triad”)
1) Canonical IDs (CIDs)

Format: cid:<namespace>:<kind>:<ULID>

Namespace defaults to your org/product (e.g., centerfire).

ULID is 26 chars, time-sortable (prefer ULID over UUID for temporal ops).

Examples:

cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW

cid:centerfire:function:01J9F80Q8SRP6ZQ1TP2TJ4JWQJ

Rules:

Generated once, stored in the entity’s descriptor (semdoc.yaml, SemBlock, etc.).

Never exposed in UX unless needed; always resolvable from slug via catalog.

2) Semantic slugs

Format: <TYPE>-<DOMAIN>-<NNN>(-<qualifier>)

TYPE in ALL-CAPS: CAP, MOD, FN, EP, EVT, TBL, TEST, ADR, AGT, TOOL, FLG, CNT (contract), CINT (ChangeIntent).

DOMAIN in ALL-CAPS short taxonomy (AUTH, GEN, RANK, BILL, DATA, ORCH, UI, etc.).

NNN is a 3-digit sequence within domain (001, 002…).

Optional -<qualifier> for subscopes (CAP-AUTH-001-session).

Examples:

Capability: CAP-AUTH-001

Module: MOD-AUTH-001-session

Function: FN-AUTH-001-issueToken

Endpoint: EP-AUTH-001-POST_v1_auth_tokens

Event: EVT-AUTH-001.auth.issued.v1

Rules:

Slugs are stable, but renameable (with alias), unlike file display names.

Use hyphen - as separator; avoid underscores.

3) Display names

Free-form titles for humans (“Authentication / Token Issuance”).

May change freely, no downstream effect.

C. Global URN/IRI for cross-system references

Use a compact project URN for graph IDs and effect references:

Graph ID (preferred internally): lexi:<Kind>/<CapSlug>/<Name>

Example: lexi:Function/CAP-AUTH-001/issueToken

Resource URNs for effects:

Databases: urn:lexi:db:CAP-BILL-001:invoices

Events: urn:lexi:event:CAP-AUTH-001:auth.issued.v1

Endpoints: urn:lexi:endpoint:CAP-AUTH-001:POST:/v1/auth/tokens

Queues/Topics: urn:lexi:topic:CAP-GEN-001:content.generated.v1

Keep both: Graph IDs for Neo4j nodes; URNs inside effects.reads/writes/calls.

D. Directory & File Naming (capability-first)
Directories

Capability folder name = slug + short CID tail for uniqueness:

CAP-AUTH-001__01J9F7Z8 (ULID first 8–10 chars)

Inside capability:

CAP-AUTH-001__01J9F7Z8/
  semdoc.yaml
  contracts/
  interfaces/
  impl/
    service/
      go/
      rust/
      node/
      py/
    lib/
      js/
      go/
      rust/
      py/
    ui/
      web-react/
      admin-react/
  tests/
  telemetry/
  flags/
  maps/
  runbooks/
  data/


Why __01J9F7Z8? Preserves human readability, guarantees uniqueness at FS level across merges/imports.

Filenames (symbols)

One primary symbol per file where practical.

Include symbol name and a graph ID tail:

TypeScript: issueToken.lexi.Function.CAP-AUTH-001.issueToken.ts

Go: issue_token.lexi.Function.CAP-AUTH-001.issueToken.go

Rust: issue_token.lexi.Function.CAP-AUTH-001.issueToken.rs

Python: issue_token.lexi.Function.CAP-AUTH-001.issueToken.py

This makes grep → graph mapping deterministic.

E. Entity-Specific Conventions
Capabilities

Slug: CAP-<DOMAIN>-<NNN> (e.g., CAP-GEN-001)

CID: cid:centerfire:capability:<ULID>

SemDoc path: /capabilities/CAP-...__<ULID8>/semdoc.yaml

Modules

Slug: MOD-<DOMAIN>-<NNN>-<qualifier>

Graph: lexi:Module/<CapSlug>/<qualifier>

Functions

Slug: FN-<DOMAIN>-<NNN>-<camelOrSnakeName> (for human surfacing)

Graph: lexi:Function/<CapSlug>/<functionName>

SemBlock carries both graph_id and cid.

Endpoints

Slug: EP-<DOMAIN>-<NNN>-<VERB>_<pathDotsOrUnderscores>

Event names are dot namespaced: auth.issued.v1 (version suffix mandatory).

Events/Telemetry

Event: EVT-<DOMAIN>-<NNN>.<namespace>.<name>.v<major>

Metric: MET-<domain>.<cap>.<name>

Logs schema: LOG-<domain>.<cap>.<name>.v<major>

Feature Flags

Slug: FLG-<DOMAIN>-<NNN>-<short> (e.g., FLG-GEN-001-persona_context_v2)

Flag key (runtime): gen.persona_context.v2 (dot-namespace, versioned)

Tests

Unit test slug: TEST-<DOMAIN>-<NNN>-<symbol>-<purpose>

File: test_<symbol>__<purpose>.lexi.Test.<CapSlug>.<symbol>.spec.ts

Contracts

Contract ID: CNT-<DOMAIN>-<NNN>-<symbolOrInterface>.v<semver>

Stored inside SemBlock: "contract_id": "CNT-AUTH-001-issueToken.v1"

Change Intents

ID: CINT-<ULID>; optional slug: CINT-<YYYYMMDD>-<short>

Canonical trailer uses the ID; slug helps humans.

Agents/Tools

Agent slug: AGT-<purpose>-<NNN> (e.g., AGT-code-refactor-001)

Tool slug: TOOL-<name>-<NNN> (e.g., TOOL-tests-001)

CIDs always present in their specs.

Episodes (experience log)

ID: EPI-<ULID>

Stored in Postgres/ClickHouse; attach {cap_slug, graph_id, cid}.

F. Versioning

APIs & Events must carry a major version suffix: .v1, .v2. No suffix means “internal draft”.

Contracts use semver: CNT-… .v1.2.0 (but we serialize v1 in names; full semver in metadata).

Adapters/Models: PSM-<capSlug>-<modelName>-<adapterVer> (e.g., PSM-CAP-GEN-001-qwen2.5-7b-lora.v3).

G. Grammar & Regex (put this in SemDoc policy)

Character set

IDs: [a-z0-9] plus separators (-, _, ., /, :).

Slugs: ^[A-Z]{2,5}(-[A-Z]{2,12}){1,3}(-[0-9]{3})(-[a-z0-9]+)*$

Example match: CAP-AUTH-001-session

Graph IDs: ^lexi:(Function|Module|Endpoint|Event|Table|Dataset|Flag|Contract)/[A-Z0-9-]+/[A-Za-z0-9._-]+$

URNs: ^urn:lexi:(db|event|endpoint|topic|queue|metric|log):[A-Z0-9-]+:[A-Za-z0-9._:/-]+$

Length limits

Slugs ≤ 80 chars, Graph IDs ≤ 160, file names ≤ 120, paths ≤ 180 (portable across OS/filesystems).

Case rules

Slugs are UPPER-SNAKE-WITH-DASHES for prefixes, lower for qualifiers.

Symbol names in file tails use original code casing.

H. Rename, Alias, and Migration Policy

No CID changes. Ever.

Slug renames create an alias record in sem-doc/catalog.yaml:

capabilities:
  - cid: cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW
    slug: CAP-AUTH-001
    aliases: [CAP-IDENTITY-001]
    display: "Authentication"


Filesystem moves: create a .id file at folder root containing the CID, so tooling resolves even if path moves.

Symlink overlay for humans (optional): keep /human/by-language/... without affecting canonical paths.

Backfill tool migrates old names to new slugs and writes alias entries.

I. Enforcement (CI/Git hooks)

Pre-commit: validate slugs, graph IDs, URNs via regex; ensure .id and SemBlock cid match.

On PR:

Cross-check SemCommit trailers Graph-IDs against changed files’ SemBlocks.

Reject effects mismatches (diff writes event:X but trailer/contract don’t).

Enforce version suffix for any new Event/Endpoint names.

Ensure folder name tail matches the capability .id ULID prefix.

On merge: update Neo4j (nodes keyed by CID, slugs as properties), upsert vector metadata with {cid, slug, graph_id}.

J. Examples (end-to-end)

Capability

Folder: CAP-GEN-001__01J9F7Z8/

CID: cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW

Display: “Content Generation”

Function

Graph ID: lexi:Function/CAP-GEN-001/buildPersonaContext

File: buildPersonaContext.lexi.Function.CAP-GEN-001.buildPersonaContext.ts

SemBlock contains:

{ "cid":"cid:centerfire:function:01J9F8P6...","graph_id":"lexi:Function/CAP-GEN-001/buildPersonaContext", ... }


Event

URN: urn:lexi:event:CAP-GEN-001:content.generated.v1

Slug: EVT-GEN-001.content.generated.v1

Flag

Slug: FLG-GEN-001-persona_context_v2

Runtime key: gen.persona_context.v2

Change Intent

ID: CINT-01J9FD3Q5S6N9Y4P0K

SemCommit includes Graph-IDs: lexi:Function/CAP-GEN-001/buildPersonaContext

K. Why not GUID-only?

Discoverability dies for humans and for lightweight tools/greps.

Opaque-only names make review, incident response, and cross-repo reasoning painful.

The dual system (slug + CID) gives you best of both: human navigation and machine permanence.

L. Drop-in SemDoc Policy (put this file at /sem-doc/policies/naming.yaml)
version: "1.0"
namespace: "centerfire"

ids:
  canonical:
    kind: ["capability","module","function","endpoint","event","table","dataset","flag","contract","test","agent","tool","change_intent","episode"]
    format: "cid:<namespace>:<kind>:<ULID>"
    ulid: { length: 26, case: "upper" }
  slugs:
    capability: { regex: "^CAP-[A-Z]{2,12}-[0-9]{3}(-[a-z0-9]+)*$", max: 80 }
    module:     { regex: "^MOD-[A-Z]{2,12}-[0-9]{3}(-[a-z0-9]+)+$", max: 80 }
    function:   { regex: "^FN-[A-Z]{2,12}-[0-9]{3}-[A-Za-z0-9._-]+$", max: 120 }
    endpoint:   { regex: "^EP-[A-Z]{2,12}-[0-9]{3}-[A-Z]+_[A-Za-z0-9._/-]+$", max: 120 }
    event:      { regex: "^EVT-[A-Z]{2,12}-[0-9]{3}\\.[a-z0-9.]+\\.v[0-9]+$", max: 120 }
    flag:       { regex: "^FLG-[A-Z]{2,12}-[0-9]{3}-[a-z0-9._-]+$", max: 120 }
    contract:   { regex: "^CNT-[A-Z]{2,12}-[0-9]{3}-[A-Za-z0-9._-]+\\.v[0-9]+$", max: 120 }
    test:       { regex: "^TEST-[A-Z]{2,12}-[0-9]{3}-[A-Za-z0-9._-]+$", max: 120 }
    agent:      { regex: "^AGT-[a-z0-9._-]+-[0-9]{3}$", max: 80 }
    tool:       { regex: "^TOOL-[a-z0-9._-]+-[0-9]{3}$", max: 80 }
  graph_id:
    regex: "^lexi:(Function|Module|Endpoint|Event|Table|Dataset|Flag|Contract)/[A-Z0-9-]+/[A-Za-z0-9._-]+$"
    max: 160
  urn:
    regex: "^urn:lexi:(db|event|endpoint|topic|queue|metric|log):[A-Z0-9-]+:[A-Za-z0-9._:/-]+$"
    max: 160

folders:
  capability_suffix: "__<ULID8>"
  id_file: ".id"     # file whose contents = canonical CID

rules:
  version_suffix_required_for_events: true
  endpoint_version_required: true
  effects_must_use_urns: true
  filenames_must_embed_graph_id_tail: true

aliases:
  allowed: true
  record_location: "sem-doc/catalog.yaml"
  require_redirect_index: true

tenancy:
  property_not_name: true

M. Summary

Use slug + CID everywhere; slugs in paths, CIDs in contracts and graphs.

Adopt graph IDs and URNs to unify code, tests, telemetry, and effects.

Bake the regex & rules into SemDoc; enforce in CI.

Provide alias + .id files for safe renames/moves.

Keep version suffixes for externally visible semantics (endpoints/events/contracts).
