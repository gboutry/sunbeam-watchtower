# Explainable Bug Search Implementation Plan

## Boundaries

- Keep refresh in `cache sync bugs`; search is read-only and strictly offline.
- Put ranking and explanations in `internal/core/service/bugsearch`.
- Reuse shared frontend workflows for API, CLI, and TUI.
- Expose stable public request/result contracts through `pkg/dto/v1`.
- Omit sensitive records without upstream calls and reject incompatible legacy
  cache schemas with a conflict response.

## Core and cache

- Extend cached bug records with comments, links, task metadata, visibility,
  sensitivity, and provenance.
- Add an explicit cache schema marker and atomically replace each fully
  hydrated project snapshot; preserve the prior snapshot on hydration failure.
- Implement normalized concepts and clauses, phrase matching, stemming, bounded
  spelling distance, compound OpenStack aliases, weighted coverage, and
  deterministic scoring.
- Score only the best occurrence of each concept across title, description,
  comments, and metadata so comment volume cannot inflate relevance.
- Support filters, regex mode, merging, alternative sorts, related results,
  grouped evidence bounded to five excerpts by default, and cache-only
  canonical direct references.

## Frontends

- Add `POST /api/v1/bugs/search` and the matching public client method.
- Add `watchtower bug search` with explicit query and filter flags plus human,
  JSON, and YAML rendering.
- Extend `bug show` to accept canonical identifiers and render full comments,
  links, tasks, sensitivity, and provenance.
- Make the Bugs TUI run list for an empty query and search for a non-empty
  query, displaying classification, score, reference, and evidence.
- Register `bug.search` as read-only, no-local-effect, embedded-capable, and
  MCP-exportable in the shared action catalogue.

## Verification

- Unit-test text, regex, fuzzy, field, filter, merge, ranking, result-class,
  excerpt, direct-reference, and no-result behavior.
- Test schema compatibility, atomic failed hydration, sensitive omission, and
  that ordinary and direct-reference searches make no tracker request.
- Test the Cinder/MicroCeph acceptance corpus, comment-volume saturation,
  default result/evidence bounds, response size, and a 1,058-document runtime
  boundary.
- Test API optional-field/error contracts and client/CLI mappings.
- Test TUI row conversion and evidence rendering.
- Run native Go tests, lint, architecture checks, changed-package coverage,
  pre-commit, and the actual CLI help/search path where the local cache permits.
