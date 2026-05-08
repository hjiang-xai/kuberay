package pgd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPGDLabelKeysPinned guards against accidental renames of the PGD-side
// label/annotation strings we mirror from github.com/xai-org/xai/cluster/
// podgroup-operator/internal/consts. PGD reads these strings verbatim; if the
// upstream constant changes and we don't notice, autoscaler-driven scale-down
// labels the wrong field and PGD silently picks wrong eviction victims.
//
// When this test fails, do NOT just update the literal: cross-check the new
// value against PGD's current `internal/consts` package and update the
// upstream constant too if the rename was intentional. Otherwise revert.
func TestPGDLabelKeysPinned(t *testing.T) {
	assert.Equal(t, "podgroup-operator.x.ai/delete-next", DeleteNextLabelKey,
		"DeleteNextLabelKey is part of PGD's stable wire contract; renaming silently breaks autoscaler scale-down")
}

// TestKubeRaySidePGDAnnotationsPinned guards the strings we own that the
// datplat-server side stamps onto RayJob.metadata.annotations. Renaming any of
// these breaks every datplat-server build that's still on the old name.
func TestKubeRaySidePGDAnnotationsPinned(t *testing.T) {
	assert.Equal(t, "dataplatform.x.ai/ray-pgd-queue", PGDQueueAnnotation)
	assert.Equal(t, "dataplatform.x.ai/ray-pgd-priority", PGDPriorityAnnotation)
	assert.Equal(t, "dataplatform.x.ai/ray-pgd-group-by-key", PGDGroupByKeyAnnotation)
}
