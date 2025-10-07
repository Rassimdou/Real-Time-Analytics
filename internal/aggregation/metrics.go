package aggregation

import (
	"sync"
	"time"
)

type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"     //valeur instantanée
	MetricTypeHistogram MetricType = "histogram" //distribution des valeurs
	MetricTypeSet       MetricType = "set"       //valeurs uniques
)

type Metric struct {
	Name      string              `json:"name"`
	Type      MetricType          `json:"type"`
	Value     float64             `json:"value"`
	Count     int64               `json:"count"`
	Timestamp time.Time           `json:"timestamp"`
	Tags      map[string]string   `json:"tags,omitempty"`
	Values    []float64           `json:"values,omitempty"` //pour histogramme
	UniqueSet map[string]struct{} `json:"-"`                //pour set
}

// newMetric crée une nouvelle métrique
func NewMetric(name string, metricType MetricType) *Metric {
	return &Metric{
		Name:      name,
		Type:      metricType,
		Timestamp: time.Now(),
		Tags:      make(map[string]string),
		Values:    make([]float64, 0),
		UniqueSet: make(map[string]struct{}),
	}
}

func (m *Metric) Increment(value float64) {
	m.Count++
	m.Value += value
	m.Timestamp = time.Now()
}

// definit une valeur pour une gauge
func (m *Metric) Set(value float64) {
	m.Value = value
	m.Timestamp = time.Now()
}

// Observe ajoute une valeur à un histogramme
func (m *Metric) Observe(value float64) {
	m.Values = append(m.Values, value)
	m.Count++
	m.Value += value
	m.Timestamp = time.Now()
}

// add unique value to a set
func (m *Metric) AddUnique(value string) {
	m.UniqueSet[value] = struct{}{}
	m.Count = int64(len(m.UniqueSet))
	m.Timestamp = time.Now()
}

func (m *Metric) Average() float64 {
	if m.Count == 0 {
		return 0
	}
	return m.Value / float64(m.Count)
}

// MetricSnapshot represente un snapshot d'une métrique à un instant donné
type MetricSnapshot struct {
	Name    string             `json:"name"`
	Metrics map[string]float64 `json:"metrics"`
	mu      sync.RWMutex
}

func NewMetricSnapshot() *MetricSnapshot {
	return &MetricSnapshot{
		Timestamp: time.Now(),
		Metrics:   make(map[string]*Metric),
	}
}

func (ms *MetricSnapshot) GetMetric(m *Metric) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if metric, exists := ms.Metrics[m.Name]; exists {
		return metric
	}

	metric := NewMetric(Name, MetricType)
	ms.Metrics[Name] = metric
	return metric

}

// GetMetricValue recupere la valeur d'une métrique
func (ms *MetricSnapshot) GetMetricValue(name string) (float64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if metric, exists := ms.Metrics[name]; exists {
		return metric.Value, true
	}
	return 0, false
}

// returns all metrics bs7 copy
func (ms *MetricSnapshot) GetAllMetrics() map[string]float64 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	//create a copy
	result := make(map[string]float64, len(ms.Metrics))
	for name, metric := range ms.Metrics {
		metricCopy := *metric
		result[name] = &metricCopy
	}
	return result
}

func (ms *MetricSnapshot) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.Metrics = make(map[string]*Metric)
	ms.Timestamp = time.Now()
}

// TimeWindow represente une fenetre de temps pour l'agrégation des métriques
type TimeWindow struct {
	Start    time.Time       `json:"start_time"`
	EndTime  time.Time       `json:"end_time"`
	Duration time.Duration   `json:"duration"`
	Metrics  *MetricSnapshot `json:"metrics"`
	Closed   bool            `json:"closed"`
}

// NewTimeWindow crée une nouvelle fenêtre de temps
func NewTimeWindow(startTime time.Time, duration time.Duration) *TimeWindow {
	return &TimeWindow{
		StartTime: startTime,
		EndTime:   startTime.Add(duration),
		Duration:  duration,
		Metrics:   NewMetricSnapshot(),
		Closed:    false,
	}
}

// IsActive verifie si la fenêtre est active
func (tw *TimeWindow) IsActive(t time.Time) bool {
	return !tw.Closed && t.After(tw.EndTime)
}

// ShouldClose verifie si la fenêtre doit être fermée
func (tw *TimeWindow) ShouldClose(t time.Time) bool {
	return !tw.Closed && t.After(tw.EndTime)
}

// Close ferme la fenêtre de temps
func (tw *TimeWindow) Close() {
	tw.Closed = true
}

// WindowManager gere les fenêtres de temps
type WindowManager struct {
	Windows  []*TimeWindow
	duration time.Duration
	mu       sync.RWMutex
}

// NewWindowManager crée un nouveau gestionnaire de fenêtres de temps
func NewWindowManager(duration time.Duration) *WindowManager {
	return &WindowManager{
		Windows:  make([]*TimeWindow, 0),
		duration: duration,
	}
}

// GetCreateWindow récupère ou crée une fenêtre de temps active
func (wm *WindowManager) GetCreateWindow(t time.Time) *TimeWindow {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Arrondir le temps au début de la fenêtre
	windowStart := t.Truncate(wm.duration)

	//chercher une fenêtre existante
	for _, window := range wm.Windows {
		if window.StartTime.Equal(windowStart) && !window.Closed {
			return window
		}
	}

	//créer une nouvelle fenêtre
	window := NewTimeWindow(windowStart, wm.duration)
	wm.Windows = append(wm.Windows, window)
	return window
}

// CloseExpiredWindows ferme les fenêtres expirées
func (wm *WindowManager) CloseExpiredWindows(t time.Time) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	closedWindows := make([]*TimeWindow, 0)

	for _, window := range wm.Windows {
		if window.ShouldClose(time.Now()) {
			window.Close()
			closedWindows = append(closedWindows, window)
		}
	}
	return closedWindows
}

// Cleanup nettoie les anciennes fenêtres fermées
func (wm *WindowManager) Cleanup(keepDuration time.Duration) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	now := time.Now()
	activeWindows := make([]*TimeWindow, 0)

	for _, window := range wm.Windows {
		// Garder si pas fermée ou fermée récemment
		if !window.Closed || now.Sub(window.EndTime) < keepDuration {
			activeWindows = append(activeWindows, window)
		}
	}

	wm.Windows = activeWindows
}

// GetActiveWindows retourne toutes les fenêtres actives
func (wm *WindowManager) GetActiveWindows() []*TimeWindow {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	active := make([]*TimeWindow, 0)
	for _, window := range wm.Windows {
		if !window.Closed {
			active = append(active, window)
		}
	}

	return active
}
