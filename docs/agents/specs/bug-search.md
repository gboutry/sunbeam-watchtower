# Functional Specification: Watchtower Bug Search

A regex-only search would not be sufficient because it requires the user to
know the wording used in the bug. A fuzzy-only search would be too opaque and
could hide relevant results. Watchtower should provide explainable text search,
with exact matching, fuzzy matching, and an optional regex mode.

## 1. Objective

Enable a human or an AI agent to determine quickly whether a bug already
exists, including when:

- the title uses different vocabulary from the diagnosis;
- the decisive information appears in the description or comments;
- several trackers contain tasks associated with the same bug;
- the bug list contains hundreds of results;
- titles displayed in the terminal are truncated.

The search must answer two questions:

1. Which bugs may correspond to the described problem?
2. Why was each bug selected as a result?

## 2. Search Scope

The search must be able to cover:

- title;
- description;
- comments;
- tags;
- project and tracker;
- status and importance;
- assignee;
- associated bug tasks;
- bug identifier;
- URL.

Users must be able to select the fields to search. By default, the search must
cover the title, description, comments, and tags.

## 3. Search Modes

### 3.1 Text Search

The default mode must accept a query containing multiple words or a complete
phrase.

Example:

```text
cinder snapshot lost during service restart
```

It must find bugs that use related wording, even when the exact terms differ,
for example:

- `volume operation interrupted`;
- `cinder-volume reconfigured`;
- `service restart race`;
- `snapshot remains in creating`.

### 3.2 Exact Search

Users must be able to request:

- an exact phrase;
- mandatory terms;
- excluded terms.

Example functional expressions:

```text
"cinder-volume"
+"relation-changed"
-noha
```

The precise query syntax may be defined during design, but these capabilities
must be available.

### 3.3 Fuzzy Search

The search must tolerate:

- grammatical variations;
- case differences;
- simple spelling mistakes;
- related forms such as `restart`, `restarted`, and `restarting`;
- closely related wording when the relationship can be explained.

Users must be able to disable fuzzy matching and request strictly lexical
matching.

### 3.4 Regular Expression Search

An optional regex mode must support advanced, deterministic searches.

It must:

- search only the selected fields;
- report invalid expressions clearly;
- distinguish no results from an invalid expression;
- allow users to choose whether matching is case-sensitive.

Regex must not be the default search mode.

### 3.5 Identifier and URL Search

A query containing a bug identifier or Launchpad URL must return the
corresponding bug directly.

## 4. Filters

Search must support the same functional filters as the existing bug list:

- project;
- tracker;
- status;
- importance;
- assignee;
- tags;
- creation or modification period.

It must also allow users to:

- include or exclude closed bugs;
- limit the number of results;
- search only bugs modified after a specified date;
- combine several filters without losing relevance ordering.

## 5. Duplicate Merging

Search must offer the same merging behavior as `bug list --merge`.

When several tasks correspond to the same bug:

- the primary result must appear only once;
- associated tasks and projects must remain visible;
- matches found in each task must be retained;
- users must be able to disable merging.

## 6. Result Ranking

Results must be ranked by relevance by default.

Ranking must functionally prioritize:

1. exact identifier or URL matches;
2. exact or very close title matches;
3. description matches;
4. comment matches;
5. tag and metadata matches.

Freshness, status, and importance must not hide a result with stronger textual
relevance. They may be used to distinguish results with comparable relevance.

Users must also be able to sort by:

- modification date;
- creation date;
- importance;
- status.

## 7. Match Explanations

Each result must explain why it matches the query.

It must provide:

- the fields that matched;
- the matching terms;
- a short, untruncated excerpt around each match;
- a relevance score or level;
- any fuzzy correspondences;
- the identifier, full title, status, project, and URL.

An AI agent must not need to open every bug individually to understand why it
is a candidate.

## 8. Output Formats

The CLI command and endpoint must expose the same information.

Required output formats are:

- human-readable output;
- JSON;
- YAML.

Structured output must contain at least:

- identifier;
- complete title;
- description or matching excerpts;
- URL;
- tracker and projects;
- status;
- importance;
- tags;
- last modification date;
- matching fields;
- relevance score or level;
- merged tasks.

Human-readable output must not silently truncate decisive information. If an
excerpt is shortened, the output must state that it has been truncated.

## 9. Natural-Language Search for AI Agents

Search must accept a complete problem description, not only keywords.

Example:

> A MicroCeph update-status republishes the same monitor addresses in a
> different order, triggers relation-changed on every cinder-volume-ceph unit,
> restarts all cinder-volume services, and loses an in-flight snapshot.

The results must identify candidates related to each important concept:

- MicroCeph;
- relation or configuration changes;
- Cinder restart;
- interrupted operation;
- snapshot;
- race condition.

It must remain possible to determine that no bug covers the complete problem,
even when several bugs cover individual parts of it.

## 10. Direct, Partial, and Related Matches

Search must distinguish between:

- a bug that directly covers the problem;
- a bug covering the same mechanism in another service;
- a bug sharing only a symptom;
- a lexical result with no evident causal relationship.

Results may be classified as:

- direct match;
- partial match;
- related result.

The classification must be justified using the matching fields and excerpts.

## 11. No-Result Behavior

When no bug matches sufficiently:

- the command must state this explicitly;
- it must not promote a weak result into a match automatically;
- it must be able to show the best related results separately;
- it must report the filters and search scope that were applied.

An AI agent must be able to conclude unambiguously that no matching bug was
found.

## 12. Access to Full Bug Details

Users must be able to retrieve full bug details from a search result, including:

- untruncated description;
- comments;
- tasks;
- per-project statuses;
- tags;
- dates;
- associated links.

This lookup must work using the identifier returned by search.

## 13. Cache and Source Consistency

Each result must indicate whether its data came from:

- the Watchtower cache;
- the remote source;
- a combination of both.

Users must be able to request a refresh before searching when freshness is
important.

Search must not present cached information as confirmed-current without
identifying its provenance.

## 14. Security and Side Effects

Search must be strictly read-only.

It must never:

- modify a bug;
- add a comment;
- change a status;
- synchronize tasks;
- create a bug.

It must respect the same access controls and confidentiality rules as the
existing bug-list operation.

## 15. Acceptance Criteria

The feature will be considered functionally complete when all of the following
scenarios succeed:

1. Searching by identifier returns the exact bug directly.
2. A sentence-level query finds a relevant bug when its title uses different
   wording.
3. A match present only in a description or comment is returned.
4. A valid regex produces deterministic results.
5. An invalid regex returns an explicit error.
6. Project, status, tag, and date filters work with every search mode.
7. Merged mode retains all associated tasks.
8. Every result explains its matches with excerpts.
9. JSON and YAML output contain complete titles and match information.
10. No results is distinguishable from an error and from related-only results.
11. Searching for the Cinder/MicroCeph incident allows the user to conclude
    that no direct bug exists while showing generic service-restart race bugs
    separately.
12. No search operation modifies Launchpad or business data in the cache.

## 16. Functional Priorities

### Required

- search across titles, descriptions, and comments;
- multi-term text search;
- exact and fuzzy matching;
- existing bug-list filters;
- result ranking and explanations;
- JSON output suitable for AI agents;
- complete titles and untruncated matching excerpts;
- distinction between direct, partial, and related results;
- explicit no-result handling.

### Desirable

- regex mode;
- field selection;
- term exclusion;
- source selection or refresh before search;
- alternative sorting modes.

### Optional

- search history;
- suggested follow-up queries;
- detailed comparison between an incident description and a candidate bug.

## 17. Approved Design Decisions

- Search is deterministic and bounded. It uses lexical and phrase matching,
  English stemming, bounded edit distance, and a small versioned OpenStack
  alias catalogue. It does not use embeddings or an external search service.
- Cache refresh remains the separate `cache sync bugs` operation. Search never
  refreshes or mutates the cache.
- The default field set is title, description, comments, and tags. Closed bugs
  are included by default.
- Search arguments use explicit CLI flags for phrases, required terms, and
  excluded terms rather than an implicit `+`/`-` query grammar.
- Search is strictly offline and never calls a tracker, including for canonical
  bug references. Sensitive cached candidates are omitted with a generic
  warning rather than revalidated during search.
- Populated bug caches carry an explicit search schema version. Search rejects
  legacy or incompatible project caches with HTTP 409; the normal
  `cache sync bugs` operation performs an atomic full rebuild.
- Text relevance counts at most one best field occurrence per query concept so
  repeated comments cannot multiply a concept's score. Classification uses
  weighted concept and clause coverage.
- Search defaults to 10 primary results, 5 related results, and 5 grouped
  evidence excerpts per result.
- The Bugs TUI is unified: an empty query lists bug tasks, while a non-empty
  query runs explainable search from the same pane.
- Search results return canonical references such as `launchpad:123`, accepted
  by `bug show` as well as the API.

## 18. Operational Clarifications

- Search results are a snapshot of the existing bug cache and report cache
  provenance and sync time. Users who need freshness run
  `watchtower cache sync bugs` first.
- Excerpts are bounded for output safety. When context is shortened, structured
  and human output explicitly mark which side was truncated.
- "Direct", "partial", and "related" classify evidence coverage. They are not
  assertions that two incidents have the same root cause.
- The initial telemetry classification is intentionally no new telemetry. The
  feature introduces no new durable operational state or live fan-out to
  measure; existing cache status remains the freshness signal.
