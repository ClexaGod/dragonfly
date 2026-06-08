package performance

import (
	"sync"
	"sync/atomic"
	"time"
)

// TransactionToken tracks the time a world transaction spends waiting before
// it starts running. Tokens should be created and completed by the same
// WorldMetrics instance.
type TransactionToken struct {
	kind     string
	queuedAt time.Time
}

// TransactionSummary contains queue wait and execution statistics for world
// transactions.
type TransactionSummary struct {
	Wait      DurationSummary
	Execution DurationSummary
}

type transactionWindow struct {
	wait      durationWindow
	execution durationWindow
}

// QueueSummary contains the current and highest observed transaction backlog.
type QueueSummary struct {
	Current int64
	Peak    int64
}

// WorldState contains lightweight gauges updated by the world tick.
type WorldState struct {
	Chunks   int64
	Entities int64
	Viewers  int64
}

// WorldSnapshot is an immutable snapshot of a world's recent performance.
type WorldSnapshot struct {
	ID           uint64
	Name         string
	Dimension    string
	TPS          float64
	Tick         DurationSummary
	Transactions TransactionSummary
	ByKind       map[string]TransactionSummary
	Operations   map[string]DurationSummary
	Queue        QueueSummary
	State        WorldState
}

// WorldMetrics records bounded, low-overhead performance measurements for one
// world. It is safe to use from multiple goroutines.
type WorldMetrics struct {
	id        uint64
	name      string
	dimension string

	mu           sync.RWMutex
	tick         durationWindow
	tickTimes    timestampWindow
	transactions transactionWindow
	byKind       map[string]*transactionWindow
	operations   map[string]*durationWindow

	queueCurrent atomic.Int64
	queuePeak    atomic.Int64
	chunks       atomic.Int64
	entities     atomic.Int64
	viewers      atomic.Int64
}

// NewWorldMetrics creates and registers performance metrics for a world.
func NewWorldMetrics(name, dimension string) *WorldMetrics {
	metrics := &WorldMetrics{
		id:         nextWorldID.Add(1),
		name:       name,
		dimension:  dimension,
		byKind:     make(map[string]*transactionWindow),
		operations: make(map[string]*durationWindow),
	}
	worlds.Store(metrics.id, metrics)
	return metrics
}

// Close removes the metrics from the global world registry.
func (m *WorldMetrics) Close() {
	worlds.Delete(m.id)
}

// BeginTransaction records a transaction entering the world queue.
func (m *WorldMetrics) BeginTransaction(kind string) TransactionToken {
	if kind == "" {
		kind = "exec"
	}
	current := m.queueCurrent.Add(1)
	for peak := m.queuePeak.Load(); current > peak && !m.queuePeak.CompareAndSwap(peak, current); peak = m.queuePeak.Load() {
	}
	return TransactionToken{kind: kind, queuedAt: time.Now()}
}

// StartTransaction records a transaction leaving the queue and returns its
// execution start time.
func (m *WorldMetrics) StartTransaction(token TransactionToken) time.Time {
	started := time.Now()
	wait := started.Sub(token.queuedAt)
	m.queueCurrent.Add(-1)

	m.mu.Lock()
	m.transactions.wait.add(wait)
	window := m.byKind[token.kind]
	if window == nil {
		window = &transactionWindow{}
		m.byKind[token.kind] = window
	}
	window.wait.add(wait)
	m.mu.Unlock()
	return started
}

// EndTransaction records the execution time of a transaction.
func (m *WorldMetrics) EndTransaction(token TransactionToken, started time.Time) {
	elapsed := time.Since(started)
	m.mu.Lock()
	m.transactions.execution.add(elapsed)
	m.byKind[token.kind].execution.add(elapsed)
	m.mu.Unlock()
}

// RecordTick records a completed world tick.
func (m *WorldMetrics) RecordTick(elapsed time.Duration, completedAt time.Time) {
	m.mu.Lock()
	m.tick.add(elapsed)
	m.tickTimes.add(completedAt)
	m.mu.Unlock()
}

// MeasureOperation starts measuring a named core operation and returns a
// function that records its duration. Operation names must be bounded,
// developer-defined values and must not contain user input.
func (m *WorldMetrics) MeasureOperation(name string) func() {
	started := time.Now()
	return func() {
		m.RecordOperation(name, time.Since(started))
	}
}

// RecordOperation records the duration of a named core operation.
func (m *WorldMetrics) RecordOperation(name string, elapsed time.Duration) {
	m.mu.Lock()
	window := m.operations[name]
	if window == nil {
		window = &durationWindow{}
		m.operations[name] = window
	}
	window.add(elapsed)
	m.mu.Unlock()
}

// SetState updates the latest lightweight world state gauges.
func (m *WorldMetrics) SetState(chunks, entities, viewers int) {
	m.chunks.Store(int64(chunks))
	m.entities.Store(int64(entities))
	m.viewers.Store(int64(viewers))
}

// Snapshot returns an immutable snapshot of the world's recent performance.
func (m *WorldMetrics) Snapshot() WorldSnapshot {
	m.mu.RLock()
	byKind := make(map[string]TransactionSummary, len(m.byKind))
	for kind, window := range m.byKind {
		byKind[kind] = TransactionSummary{Wait: window.wait.summary(), Execution: window.execution.summary()}
	}
	operations := make(map[string]DurationSummary, len(m.operations))
	for name, window := range m.operations {
		operations[name] = window.summary()
	}
	snapshot := WorldSnapshot{
		ID:           m.id,
		Name:         m.name,
		Dimension:    m.dimension,
		TPS:          m.tickTimes.ticksPerSecond(),
		Tick:         m.tick.summary(),
		Transactions: TransactionSummary{Wait: m.transactions.wait.summary(), Execution: m.transactions.execution.summary()},
		ByKind:       byKind,
		Operations:   operations,
	}
	m.mu.RUnlock()

	snapshot.Queue = QueueSummary{Current: m.queueCurrent.Load(), Peak: m.queuePeak.Load()}
	snapshot.State = WorldState{Chunks: m.chunks.Load(), Entities: m.entities.Load(), Viewers: m.viewers.Load()}
	return snapshot
}
