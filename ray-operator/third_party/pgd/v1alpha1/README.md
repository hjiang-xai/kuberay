# Vendored PodGroupDeployment (PGD) Types

This directory contains a vendored copy of the `apps.x.ai/v1alpha1` API types from xAI's
`podgroup-operator`. We vendor (rather than module-import) because the upstream lives in a
private monorepo; vendoring keeps this fork buildable without cross-repo Go module setup.

## Source of truth

- Upstream package: `github.com/xai-org/xai/cluster/podgroup-operator/api/v1alpha1`
- Source commit:    `ba2e493b08387fbdd94b03682f528e9f366a9b35`
- Date pinned:      2026-04-23
- Files copied verbatim:
  - `groupversion_info.go`
  - `podgroupdeployment_types.go` (resource-helper methods stripped — see below)
  - `nodepriorities_types.go`
  - `queue_types.go`
  - `queueacl_types.go`
  - `resource_types.go`
  - `scheduling_detail.go`
  - `zz_generated.deepcopy.go`

## Local modifications

- `podgroupdeployment_types.go`: removed `GPUsPerGroup()`, `CPUsPerGroup()`,
  `MemoryPerGroup()` and the `k8s.io/component-helpers/resource` import. These helpers
  are PGD-internal and would pull a heavier dependency into KubeRay. Our fork doesn't
  use them; PGD's own scheduler does the resource accounting.

## Refresh procedure

When PGD's API changes:

1. Compare the upstream `api/v1alpha1/` against this directory.
2. Copy changed files verbatim.
3. Re-strip the three `*PerGroup` helpers from `podgroupdeployment_types.go` if
   they reappear, plus the `component-helpers/resource` and `strings` imports.
4. Update the source commit hash above.
5. Run `make test` in `ray-operator/` to confirm no breakage.

## Why not use a Go module replace directive

A `replace github.com/xai-org/xai/cluster/podgroup-operator => /local/path` works for
local dev but breaks CI (the upstream module isn't published). Vendoring is the
simplest option that works in all environments.
