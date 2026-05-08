package pgd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// PGD names are validated by a kubebuilder rule on PodGroupDeployment:
//
//	metadata.name must be at most 54 characters
//
// (see ray-operator/third_party/pgd/v1alpha1/podgroupdeployment_types.go).
//
// PGD then derives pod names as `<pgdName>-<rand5>-<ordinal>` (see
// cluster/podgroup-operator/internal/scheduler/reconcile.go), which adds
// up to 1+5+1+(digits of ordinal) = ~10 chars. Pod names must be ≤ 63 chars
// total, so a 54-char PGD name leaves enough room.
const maxPGDNameLen = 54

// HeadPGDName returns the deterministic PGD name for a RayCluster's head pod
// group. Format: `<rayCluster>-h`. Truncated with a hash suffix if it would
// exceed the 54-char limit.
func HeadPGDName(rayClusterName string) string {
	return capName(rayClusterName + "-h")
}

// WorkerPGDName returns the deterministic PGD name for a RayCluster's worker
// pod group. Format: `<rayCluster>-w-<groupName>`. Truncated with a hash
// suffix if it would exceed the 54-char limit.
func WorkerPGDName(rayClusterName, groupName string) string {
	return capName(fmt.Sprintf("%s-w-%s", rayClusterName, groupName))
}

// capName returns name unchanged if ≤ maxPGDNameLen, otherwise truncates
// the prefix to leave room for a 9-char hash suffix (`-<8hex>`) so the
// result is uniquely tied to the original name. The hash is over the full
// pre-truncation name so distinct inputs always produce distinct outputs.
func capName(name string) string {
	if len(name) <= maxPGDNameLen {
		return name
	}
	h := sha256.Sum256([]byte(name))
	suffix := "-" + hex.EncodeToString(h[:4]) // 8 hex chars
	prefix := name[:maxPGDNameLen-len(suffix)]
	return prefix + suffix
}
