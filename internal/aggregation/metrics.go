package aggregation

import (
    "sync"
    "time"
)

type MetricType string

const (
    MetricTypeCounter   MetricType = "counter"
    MetricTypeGauge     MetricType = "gauge"     // valeur instantanée
    MetricTypeHistogram MetricType = "histogram" // distribution des valeurs
    MetricTypeSet       MetricType = "set"       // valeurs uniques
)

type Metric struct {
    Name      string              `json:"name"`
    Type      MetricType          `json:"type"`
    Value     float64             `json:"value"`
    Count     int64               `json:"count"`
    Timestamp time.Time           `json:"timestamp"`
    Tags      map[string]string   `json:"tags,omitempty"`
    Values    []float64           `json:"values,omitempty"` // pour histogramme
    UniqueSet map[string]struct{} `json:"-"`                // pour set
    mu        sync.RWMutex                                 // protège les champs ci-dessus
}

  // NewMetric crée une nouvelle métrique
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
  // Increment augmente une métrique compteur de 1
  func (m *Metric) Increment() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Count++
    m.Value += 1
    m.Timestamp = time.Now()
  }

  // IncrementBy augmente une métrique compteur d'une valeur donnée
  func (m *Metric) IncrementBy(value float64) {
	// Pour un comptreur numérique, on additionne la valeur
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Value += value
	m.Count++
	m.Timestamp = time.Now()
}

// Set définit une valeur pour une gauge
func (m *Metric) Set(value float64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Value = value
    m.Timestamp = time.Now()
}

// Observe ajoute une valeur à un histogramme
func (m *Metric) Observe(value float64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Values = append(m.Values, value)
    m.Count++
    m.Value += value
    m.Timestamp = time.Now()
}

// add unique value to a set
func (m *Metric) AddUnique(value string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.UniqueSet[value] = struct{}{}
    m.Count = int64(len(m.UniqueSet))
    m.Timestamp = time.Now()
}

func (m *Metric) Average() float64 {
    m.mu.RLock()
    defer m.mu.RUnlock()
    if m.Count == 0 {
        return 0
    }
    return m.Value / float64(m.Count)
}

// MetricsSnapshot represente un ensemble de métriques à un instant donné
type MetricsSnapshot struct {
    Metrics   map[string]*Metric `json:"metrics"`
    Timestamp time.Time          `json:"timestamp"`
    mu        sync.RWMutex
}

func NewMetricsSnapshot() *MetricsSnapshot {
    return &MetricsSnapshot{
        Timestamp: time.Now(),
        Metrics:   make(map[string]*Metric),
    }
}

// GetMetric récupère ou crée une métrique par nom et type
func (ms *MetricsSnapshot) GetMetric(name string, metricType MetricType) *Metric {
	ms.mu.Lock()
	defer ms.mu.Unlock()
 
	if metric, exists := ms.Metrics[name]; exists {
		return metric
	}

	metric := NewMetric(name, metricType)
	ms.Metrics[name] = metric
	return metric
}

// GetMetricValue recupere la valeur d'une métrique
func (ms *MetricsSnapshot) GetMetricValue(name string) (float64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
 
	if metric, exists := ms.Metrics[name]; exists {
		return metric.Value, true
	}
	return 0, false
}

// GetAllMetrics retourne toutes les métriques (copie superficielle du map)
func (ms *MetricsSnapshot) GetAllMetrics() map[string]*Metric {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
 
	result := make(map[string]*Metric, len(ms.Metrics))
	for name, metric := range ms.Metrics {
		result[name] = metric
	}
	return result
}

func (ms *MetricsSnapshot) Reset() {
    ms.mu.Lock()
    defer ms.mu.Unlock()
 
    ms.Metrics = make(map[string]*Metric)
    ms.Timestamp = time.Now()
}

// TimeWindow represente une fenetre de temps pour l'agrégation des métriques
type TimeWindow struct {
    StartTime time.Time      `json:"start_time"`
    EndTime  time.Time      `json:"end_time"`
    Duration time.Duration   `json:"duration"`
    Metrics  *MetricsSnapshot `json:"metrics"`
    Closed   bool            `json:"closed"`
}

// NewTimeWindow crée une nouvelle fenêtre de temps
func NewTimeWindow(startTime time.Time, duration time.Duration) *TimeWindow {
	return &TimeWindow{
		StartTime: startTime,
		EndTime:   startTime.Add(duration),
		Duration:  duration,
		Metrics:   NewMetricsSnapshot(),
		Closed:    false,
	}
}

// IsActive verifie si la fenêtre est active
func (tw *TimeWindow) IsActive(t time.Time) bool {
	return !tw.Closed && t.Before(tw.EndTime)
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

// GetOrCreateWindow récupère ou crée une fenêtre de temps active
func (wm *WindowManager) GetOrCreateWindow(t time.Time) *TimeWindow {
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
func (wm *WindowManager) CloseExpiredWindows(t time.Time) []*TimeWindow {
	wm.mu.Lock()
	defer wm.mu.Unlock()
 
	closedWindows := make([]*TimeWindow, 0)
 
	for _, window := range wm.Windows {
		if window.ShouldClose(t) {
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
