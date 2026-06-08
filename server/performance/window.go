package performance

import (
	"slices"
	"time"
)

const sampleWindowSize = 1200

// DurationSummary contains statistics calculated over the most recent samples
// in a fixed-size window. Count is the total number of samples recorded since
// the metric was created.
type DurationSummary struct {
	Count   uint64
	Samples int
	Last    time.Duration
	Average time.Duration
	P95     time.Duration
	P99     time.Duration
	Maximum time.Duration
}

type durationWindow struct {
	values [sampleWindowSize]time.Duration
	next   int
	size   int
	count  uint64
}

func (w *durationWindow) add(value time.Duration) {
	w.values[w.next] = value
	w.next = (w.next + 1) % len(w.values)
	if w.size < len(w.values) {
		w.size++
	}
	w.count++
}

func (w *durationWindow) summary() DurationSummary {
	if w.size == 0 {
		return DurationSummary{Count: w.count}
	}

	values := make([]time.Duration, w.size)
	start := (w.next - w.size + len(w.values)) % len(w.values)
	for i := range values {
		values[i] = w.values[(start+i)%len(w.values)]
	}

	last := values[len(values)-1]
	var total time.Duration
	for _, value := range values {
		total += value
	}

	slices.Sort(values)
	return DurationSummary{
		Count:   w.count,
		Samples: len(values),
		Last:    last,
		Average: total / time.Duration(len(values)),
		P95:     percentile(values, 0.95),
		P99:     percentile(values, 0.99),
		Maximum: values[len(values)-1],
	}
}

func percentile(values []time.Duration, percentile float64) time.Duration {
	index := int(float64(len(values))*percentile+0.999999) - 1
	return values[max(0, min(index, len(values)-1))]
}

type timestampWindow struct {
	values [sampleWindowSize]int64
	next   int
	size   int
}

func (w *timestampWindow) add(value time.Time) {
	w.values[w.next] = value.UnixNano()
	w.next = (w.next + 1) % len(w.values)
	if w.size < len(w.values) {
		w.size++
	}
}

func (w *timestampWindow) ticksPerSecond() float64 {
	if w.size == 0 {
		return 0
	}
	if w.size == 1 {
		return 20
	}

	start := (w.next - w.size + len(w.values)) % len(w.values)
	first, last := w.values[start], w.values[(w.next-1+len(w.values))%len(w.values)]
	elapsed := time.Duration(last - first)
	if elapsed <= 0 {
		return 20
	}
	return min(20, float64(w.size-1)/elapsed.Seconds())
}
