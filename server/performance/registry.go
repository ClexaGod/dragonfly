package performance

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
)

var (
	nextWorldID atomic.Uint64
	worlds      sync.Map
)

// RuntimeSnapshot contains process-wide runtime information.
type RuntimeSnapshot struct {
	Goroutines  int
	HeapAlloc   uint64
	HeapInUse   uint64
	HeapObjects uint64
	TotalAlloc  uint64
	GCCycles    uint32
	LastGCPause uint64
	NextGC      uint64
	Worlds      int
}

// WorldSnapshots returns snapshots of all currently registered worlds.
func WorldSnapshots() []WorldSnapshot {
	snapshots := make([]WorldSnapshot, 0, 3)
	worlds.Range(func(_, value any) bool {
		snapshots = append(snapshots, value.(*WorldMetrics).Snapshot())
		return true
	})
	slices.SortFunc(snapshots, func(a, b WorldSnapshot) int {
		if a.Name == b.Name {
			return compare(a.Dimension, b.Dimension)
		}
		return compare(a.Name, b.Name)
	})
	return snapshots
}

// Runtime returns a snapshot of process-wide Go runtime information.
func Runtime() RuntimeSnapshot {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	worldCount := 0
	worlds.Range(func(_, _ any) bool {
		worldCount++
		return true
	})
	return RuntimeSnapshot{
		Goroutines:  runtime.NumGoroutine(),
		HeapAlloc:   memory.HeapAlloc,
		HeapInUse:   memory.HeapInuse,
		HeapObjects: memory.HeapObjects,
		TotalAlloc:  memory.TotalAlloc,
		GCCycles:    memory.NumGC,
		LastGCPause: memory.PauseNs[(memory.NumGC+255)%256],
		NextGC:      memory.NextGC,
		Worlds:      worldCount,
	}
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
