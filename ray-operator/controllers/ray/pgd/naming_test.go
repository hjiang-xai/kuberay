package pgd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadPGDName(t *testing.T) {
	tests := []struct {
		name           string
		rayClusterName string
		wantSuffix     string
		wantHashed     bool
	}{
		{name: "short", rayClusterName: "myjob", wantSuffix: "myjob-h", wantHashed: false},
		{name: "exact-fit", rayClusterName: strings.Repeat("a", maxPGDNameLen-2), wantSuffix: strings.Repeat("a", maxPGDNameLen-2) + "-h", wantHashed: false},
		{name: "one-over", rayClusterName: strings.Repeat("a", maxPGDNameLen-1), wantHashed: true},
		{name: "very-long", rayClusterName: strings.Repeat("a", 200), wantHashed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HeadPGDName(tt.rayClusterName)
			assert.LessOrEqual(t, len(got), maxPGDNameLen, "must respect 54-char cap")
			if !tt.wantHashed {
				assert.Equal(t, tt.wantSuffix, got)
			} else {
				assert.Len(t, got, maxPGDNameLen, "hashed name should fill the cap exactly")
			}
		})
	}
}

func TestWorkerPGDName(t *testing.T) {
	tests := []struct {
		name           string
		rayClusterName string
		groupName      string
		want           string
		wantHashed     bool
	}{
		{name: "short", rayClusterName: "myjob", groupName: "worker", want: "myjob-w-worker", wantHashed: false},
		{name: "short cpu group", rayClusterName: "myjob", groupName: "cpu-worker", want: "myjob-w-cpu-worker", wantHashed: false},
		{name: "long cluster", rayClusterName: strings.Repeat("a", 50), groupName: "worker", wantHashed: true},
		{name: "long group", rayClusterName: "myjob", groupName: strings.Repeat("g", 100), wantHashed: true},
		{name: "very long both", rayClusterName: strings.Repeat("a", 200), groupName: strings.Repeat("g", 200), wantHashed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorkerPGDName(tt.rayClusterName, tt.groupName)
			assert.LessOrEqual(t, len(got), maxPGDNameLen, "must respect 54-char cap")
			if !tt.wantHashed {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Len(t, got, maxPGDNameLen, "hashed name should fill the cap exactly")
			}
		})
	}
}

// TestNamingDeterministic ensures repeated calls produce identical names so
// the helper's CreateOrPatch addresses the same PGD CR each reconcile. We
// snapshot once and compare every subsequent call against the snapshot to
// catch any non-determinism (e.g. someone replacing sha256 with a randomized
// hash).
func TestNamingDeterministic(t *testing.T) {
	long := strings.Repeat("x", 200)
	headShort := HeadPGDName("myjob")
	workerShort := WorkerPGDName("myjob", "worker")
	headLong := HeadPGDName(long)
	workerLong := WorkerPGDName(long, "worker")
	for range 100 {
		assert.Equal(t, headShort, HeadPGDName("myjob"))
		assert.Equal(t, workerShort, WorkerPGDName("myjob", "worker"))
		assert.Equal(t, headLong, HeadPGDName(long))
		assert.Equal(t, workerLong, WorkerPGDName(long, "worker"))
	}
}

// TestNamingUnique ensures distinct inputs produce distinct outputs even when
// truncated. The hash suffix is the only differentiator at the boundary.
func TestNamingUnique(t *testing.T) {
	a := HeadPGDName(strings.Repeat("a", 200))
	b := HeadPGDName(strings.Repeat("b", 200))
	assert.NotEqual(t, a, b, "different long inputs must hash to different outputs")

	c := WorkerPGDName(strings.Repeat("a", 100), "g1")
	d := WorkerPGDName(strings.Repeat("a", 100), "g2")
	assert.NotEqual(t, c, d, "different group names with same prefix must hash to different outputs")
}

func TestCapName(t *testing.T) {
	t.Run("under cap returns unchanged", func(t *testing.T) {
		s := strings.Repeat("x", maxPGDNameLen-1)
		assert.Equal(t, s, capName(s))
	})
	t.Run("at cap returns unchanged", func(t *testing.T) {
		s := strings.Repeat("x", maxPGDNameLen)
		assert.Equal(t, s, capName(s))
	})
	t.Run("over cap is hashed and capped", func(t *testing.T) {
		s := strings.Repeat("x", maxPGDNameLen+1)
		got := capName(s)
		assert.Len(t, got, maxPGDNameLen)
		assert.Contains(t, got, "-")
		// last 9 chars must be -<8hex>
		suffix := got[len(got)-9:]
		require.Equal(t, "-", suffix[:1])
		for _, c := range suffix[1:] {
			assert.Contains(t, "0123456789abcdef", string(c), "suffix must be hex")
		}
	})
}
