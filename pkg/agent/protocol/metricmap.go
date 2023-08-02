package protocol

import "sync"

// MetricMap is a simple storage for name-indexed counter metrics.
type MetricMap struct {
	metrics map[string]uint
	mutex   sync.RWMutex
}

// Inc increases the value of the specified counter by one.
func (m *MetricMap) Inc(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.metrics == nil {
		m.metrics = make(map[string]uint)
	}

	m.metrics[name]++
}

// Map returns a map of the counters indexed by name. The returned map is a copy of the internal storage.
func (m *MetricMap) Map() map[string]uint {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	out := make(map[string]uint, len(m.metrics))
	for k, v := range m.metrics {
		out[k] = v
	}

	return out
}
