package builtin

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/performance"
	"github.com/df-mc/dragonfly/server/world"
)

var registerPerformanceOnce sync.Once

// RegisterPerformance registers built-in performance inspection commands.
func RegisterPerformance() {
	registerPerformanceOnce.Do(func() {
		cmd.Register(cmd.New("tps", "Displays TPS and timing metrics for the current world.", nil, tpsCommand{}))
		cmd.Register(cmd.New("status", "Displays server runtime and world performance status.", nil, statusCommand{}))
	})
}

type tpsCommand struct{}

func (tpsCommand) Run(_ cmd.Source, o *cmd.Output, tx *world.Tx) {
	snapshot := tx.World().Metrics().Snapshot()

	o.Printf("World: %s (%s)", snapshot.Name, snapshot.Dimension)
	o.Printf("TPS: %.2f | MSPT avg/p95/p99/max: %s / %s / %s / %s",
		snapshot.TPS,
		formatDuration(snapshot.Tick.Average),
		formatDuration(snapshot.Tick.P95),
		formatDuration(snapshot.Tick.P99),
		formatDuration(snapshot.Tick.Maximum),
	)
	o.Printf("Queue: current %d, peak %d | wait avg/p95/max: %s / %s / %s",
		snapshot.Queue.Current,
		snapshot.Queue.Peak,
		formatDuration(snapshot.Transactions.Wait.Average),
		formatDuration(snapshot.Transactions.Wait.P95),
		formatDuration(snapshot.Transactions.Wait.Maximum),
	)
	o.Printf("Transactions avg/p95/max: %s / %s / %s",
		formatDuration(snapshot.Transactions.Execution.Average),
		formatDuration(snapshot.Transactions.Execution.P95),
		formatDuration(snapshot.Transactions.Execution.Maximum),
	)

	printOperationSummaries(o, snapshot.Operations)
}

type statusCommand struct{}

func (statusCommand) Run(_ cmd.Source, o *cmd.Output, _ *world.Tx) {
	runtime := performance.Runtime()
	snapshots := performance.WorldSnapshots()

	o.Printf("Runtime: goroutines %d | heap %s/%s | objects %d | GC cycles %d | last GC pause %s",
		runtime.Goroutines,
		formatBytes(runtime.HeapAlloc),
		formatBytes(runtime.HeapInUse),
		runtime.HeapObjects,
		runtime.GCCycles,
		formatDuration(time.Duration(runtime.LastGCPause)),
	)
	o.Printf("Worlds: %d | total allocated: %s | next GC: %s",
		runtime.Worlds,
		formatBytes(runtime.TotalAlloc),
		formatBytes(runtime.NextGC),
	)

	for _, snapshot := range snapshots {
		o.Printf("%s (%s): TPS %.2f | MSPT avg/p95 %s/%s | queue %d/%d | chunks %d | entities %d | viewers %d",
			snapshot.Name,
			snapshot.Dimension,
			snapshot.TPS,
			formatDuration(snapshot.Tick.Average),
			formatDuration(snapshot.Tick.P95),
			snapshot.Queue.Current,
			snapshot.Queue.Peak,
			snapshot.State.Chunks,
			snapshot.State.Entities,
			snapshot.State.Viewers,
		)
	}
}

func printOperationSummaries(o *cmd.Output, operations map[string]performance.DurationSummary) {
	names := make([]string, 0, len(operations))
	for name, summary := range operations {
		if summary.Count > 0 {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		summary := operations[name]
		o.Printf("%s avg/p95/max: %s / %s / %s (%d samples)",
			name,
			formatDuration(summary.Average),
			formatDuration(summary.P95),
			formatDuration(summary.Maximum),
			summary.Samples,
		)
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0ms"
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fus", float64(d)/float64(time.Microsecond))
	default:
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
}

func formatBytes(bytes uint64) string {
	const mib = 1024 * 1024
	return fmt.Sprintf("%.1f MiB", float64(bytes)/mib)
}
