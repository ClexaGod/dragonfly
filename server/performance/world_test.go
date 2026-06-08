package performance

import (
	"testing"
	"time"
)

func TestWorldMetricsSnapshot(t *testing.T) {
	metrics := NewWorldMetrics("test", "overworld")
	defer metrics.Close()

	token := metrics.BeginTransaction("world_tick")
	time.Sleep(time.Millisecond)
	started := metrics.StartTransaction(token)
	time.Sleep(time.Millisecond)
	metrics.EndTransaction(token, started)
	metrics.RecordTick(2*time.Millisecond, time.Now())
	metrics.SetState(4, 3, 2)
	metrics.RecordOperation("chunk_load", 5*time.Millisecond)

	snapshot := metrics.Snapshot()
	if snapshot.Queue.Current != 0 || snapshot.Queue.Peak != 1 {
		t.Fatalf("unexpected queue summary: %+v", snapshot.Queue)
	}
	if snapshot.State != (WorldState{Chunks: 4, Entities: 3, Viewers: 2}) {
		t.Fatalf("unexpected world state: %+v", snapshot.State)
	}
	if snapshot.Transactions.Execution.Count != 1 || snapshot.ByKind["world_tick"].Wait.Count != 1 {
		t.Fatalf("transaction was not recorded: %+v", snapshot.Transactions)
	}
	if snapshot.Operations["chunk_load"].Count != 1 {
		t.Fatalf("operation was not recorded: %+v", snapshot.Operations)
	}
}

func TestDurationWindowIsBounded(t *testing.T) {
	var window durationWindow
	for i := 0; i < sampleWindowSize+10; i++ {
		window.add(time.Duration(i))
	}
	summary := window.summary()
	if summary.Count != sampleWindowSize+10 || summary.Samples != sampleWindowSize {
		t.Fatalf("unexpected bounded summary: %+v", summary)
	}
}
