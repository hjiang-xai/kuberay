# Vendored PodGroupDeployment (PGD) Types

This directory contains a vendored copy of the `apps.x.ai/v1alpha1` API types from xAI's
`podgroup-operator`. We vendor (rather than module-import) because the upstream lives in a
private monorepo; vendoring keeps this fork buildable without cross-repo Go module setup.

## Source of truth

- Upstream package: `github.com/xai-org/xai/cluster/podgroup-operator/api/v1alpha1`
- Source commit:    `ba2e493b08387fbdd94b03682f528e9f366a9b35`
- Date pinned:      2026-04-23
- Files copied:
  - `groupversion_info.go`            (verbatim)
  - `podgroupdeployment_types.go`     (verbatim except for the three resource-helper
                                       methods stripped — see below)
- Files generated locally:
  - `zz_generated.deepcopy.go`        (controller-gen, run over the slimmed
                                       package — only contains methods for
                                       PodGroupDeployment & friends)

We deliberately do NOT vendor the other PGD CRD types (`Queue`, `QueueACL`,
`NodePriorities`, `Resource`, `SchedulingDetail`, etc.). Our fork only
reads/writes `PodGroupDeployment`; the other CRDs are managed by the PGD
operator itself, and pulling them in would only inflate the vendor surface.
If a future change needs one of these types, copy the source file in and
re-run `controller-gen` (see refresh procedure below).

## Local modifications

- `podgroupdeployment_types.go`: removed `GPUsPerGroup()`, `CPUsPerGroup()`,
  `MemoryPerGroup()` and the `k8s.io/component-helpers/resource` import. These helpers
  are PGD-internal and would pull a heavier dependency into KubeRay. Our fork doesn't
  use them; PGD's own scheduler does the resource accounting.

## Refresh procedure

When PGD's `PodGroupDeployment` API changes:

1. From the xai monorepo root, copy `cluster/podgroup-operator/api/v1alpha1/podgroupdeployment_types.go`
   into this directory, replacing the existing file.
2. Re-strip the three `*PerGroup` helpers (`GPUsPerGroup`, `CPUsPerGroup`,
   `MemoryPerGroup`) and the `k8s.io/component-helpers/resource` and
   `strings` imports — they pull a heavier dependency we don't need.
3. From `ray-operator/`, regenerate the deepcopy file:
   ```
   make controller-gen
   ./bin/controller-gen object paths="./third_party/pgd/v1alpha1/..."
   ```
4. Update the source commit hash above to the xai monorepo commit you copied
   from.
5. Run `go build ./...` and `make test` in `ray-operator/` to confirm no
   breakage.

## Why not use a Go module replace directive

A `replace github.com/xai-org/xai/cluster/podgroup-operator => /local/path` works for
local dev but breaks CI (the upstream module isn't published). Vendoring is the
simplest option that works in all environments.
