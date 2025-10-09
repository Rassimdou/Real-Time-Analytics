package aggregation

import (
	"fmt"
	"math"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestMetricIncrement teste l'incrémentation d'un compteur
func TestMetricIncrement(t *testing.T) {
	metric := NewMetric(fmt.Sprintf("test_metric_%d", time.Now().UnixNano()), MetricTypeCounter)

	// Incrémenter 10 fois
	for i := 0; i < 10; i++ {
		metric.Increment()
	}

	// Vérifications
	if metric.Count != 10 {
		t.Errorf("Expected count 10, got %d", metric.Count)
	}

	if metric.Value != 10 {
		t.Errorf("Expected value 10, got %f", metric.Value)
	}
}

// TestMetricIncrementBy teste l'incrémentation par valeur
func TestMetricIncrementBy(t *testing.T) {
	metric := NewMetric("revenue", MetricTypeCounter)

	// Ajouter des revenues
	metric.IncrementBy(99.99)
	metric.IncrementBy(149.99)
	metric.IncrementBy(49.99)

	expectedValue := 299.97
	if metric.Value != expectedValue {
		t.Errorf("Expected value %.2f, got %.2f", expectedValue, metric.Value)
	}

	if metric.Count != 3 {
		t.Errorf("Expected count 3, got %d", metric.Count)
	}
}

// TestMetricAverage teste le calcul de moyenne
func TestMetricAverage(t *testing.T) {
	metric := NewMetric("avg_test", MetricTypeCounter)

	metric.IncrementBy(10)
	metric.IncrementBy(20)
	metric.IncrementBy(30)

	avg := metric.Average()
	expectedAvg := 20.0

	if avg != expectedAvg {
		t.Errorf("Expected average %.2f, got %.2f", expectedAvg, avg)
	}
}

// TestMetricHistogram teste l'observation de valeurs
func TestMetricHistogram(t *testing.T) {
	metric := NewMetric("histogram_test", MetricTypeHistogram)

	values := []float64{10, 20, 30, 40, 50}
	for _, v := range values {
		metric.Observe(v)
	}

	if metric.Count != int64(len(values)) {
		t.Errorf("Expected count %d, got %d", len(values), metric.Count)
	}

	if len(metric.Values) != len(values) {
		t.Errorf("Expected %d values, got %d", len(values), len(metric.Values))
	}
}

// TestMetricsSnapshotGetMetric teste la récupération/création de métrique
func TestMetricsSnapshotGetMetric(t *testing.T) {
	snapshot := NewMetricsSnapshot()

	// Première fois: création
	metric1 := snapshot.GetMetric("pageviews", MetricTypeCounter)
	if metric1 == nil {
		t.Fatal("Expected metric, got nil")
	}

	metric1.Increment()

	// Deuxième fois: récupération
	metric2 := snapshot.GetMetric("pageviews", MetricTypeCounter)
	if metric2.Count != 1 {
		t.Errorf("Expected count 1, got %d", metric2.Count)
	}

	// Vérifier que c'est la même métrique
	if metric1 != metric2 {
		t.Error("Expected same metric instance")
	}
}

// TestMetricsSnapshotUnique teste le comptage unique
func TestMetricsSnapshotUnique(t *testing.T) {
	snapshot := NewMetricsSnapshot()

	uniqueUsers := snapshot.GetMetric("unique_users", MetricTypeSet)

	// Ajouter les mêmes utilisateurs plusieurs fois
	uniqueUsers.AddUnique("user_1")
	uniqueUsers.AddUnique("user_2")
	uniqueUsers.AddUnique("user_1") // Doublon
	uniqueUsers.AddUnique("user_3")
	uniqueUsers.AddUnique("user_2") // Doublon

	// Doit compter 3 utilisateurs uniques
	if uniqueUsers.Count != 3 {
		t.Errorf("Expected count 3, got %d", uniqueUsers.Count)
	}
}

// TestTimeWindowIsActive teste si une fenêtre est active
func TestTimeWindowIsActive(t *testing.T) {
	startTime := time.Now()
	window := NewTimeWindow(startTime, 1*time.Minute)

	// Temps dans la fenêtre
	timeInWindow := startTime.Add(30 * time.Second)
	if !window.IsActive(timeInWindow) {
		t.Error("Expected window to be active")
	}

	// Temps après la fenêtre
	timeAfterWindow := startTime.Add(2 * time.Minute)
	if window.IsActive(timeAfterWindow) {
		t.Error("Expected window to be inactive")
	}

	// Après fermeture
	window.Close()
	if window.IsActive(timeInWindow) {
		t.Error("Expected closed window to be inactive")
	}
}

// TestTimeWindowShouldClose teste la détection de fermeture
func TestTimeWindowShouldClose(t *testing.T) {
	startTime := time.Now()
	window := NewTimeWindow(startTime, 1*time.Minute)

	// Avant la fin
	timeBeforeEnd := startTime.Add(59 * time.Second)
	if window.ShouldClose(timeBeforeEnd) {
		t.Error("Expected window not to close yet")
	}

	// Après la fin
	timeAfterEnd := startTime.Add(61 * time.Second)
	if !window.ShouldClose(timeAfterEnd) {
		t.Error("Expected window to close")
	}
}

// TestWindowManagerGetOrCreate teste la gestion de fenêtres
func TestWindowManagerGetOrCreate(t *testing.T) {
	wm := NewWindowManager(1 * time.Minute)

	now := time.Now()

	// Créer première fenêtre
	window1 := wm.GetOrCreateWindow(now)
	if window1 == nil {
		t.Fatal("Expected window, got nil")
	}

	// Même fenêtre (même minute)
	window2 := wm.GetOrCreateWindow(now.Add(30 * time.Second))
	if window1 != window2 {
		t.Error("Expected same window")
	}

	// Nouvelle fenêtre (minute suivante)
	window3 := wm.GetOrCreateWindow(now.Add(61 * time.Second))
	if window1 == window3 {
		t.Error("Expected different window")
	}
}

// TestAggregatorProcessEvent teste le traitement d'événement
func TestAggregatorProcessEvent(t *testing.T) {
	logger := zap.NewNop()
	agg := NewAggregator(1*time.Minute, 10*time.Second, logger)

	// Traiter un événement
	event := Event{
		ID:        "evt_1",
		Type:      "pageview",
		Timestamp: time.Now(),
		UserID:    "user_1",
		Properties: map[string]interface{}{
			"page": "/home",
		},
	}

	agg.ProcessEvent(event)

	// Vérifier les métriques
	metrics := agg.GetGlobalMetrics()

	if val, ok := metrics["total_events"]; !ok || val.Count != 1 {
		t.Error("Expected total_events to be 1")
	}

	if val, ok := metrics["pageviews"]; !ok || val.Count != 1 {
		t.Error("Expected pageviews to be 1")
	}

	if val, ok := metrics["unique_users"]; !ok || val.Count != 1 {
		t.Error("Expected 1 unique user")
	}
}

// TestAggregatorMultipleEvents teste plusieurs événements
func TestAggregatorMultipleEvents(t *testing.T) {
	logger := zap.NewNop()
	agg := NewAggregator(1*time.Minute, 10*time.Second, logger)

	// Traiter plusieurs événements
	events := []Event{
		{
			ID:         "evt_1",
			Type:       "pageview",
			Timestamp:  time.Now(),
			UserID:     "user_1",
			Properties: map[string]interface{}{"page": "/home"},
		},
		{
			ID:         "evt_2",
			Type:       "pageview",
			Timestamp:  time.Now(),
			UserID:     "user_2",
			Properties: map[string]interface{}{"page": "/products"},
		},
		{
			ID:         "evt_3",
			Type:       "click",
			Timestamp:  time.Now(),
			UserID:     "user_1",
			Properties: map[string]interface{}{"element": "button"},
		},
		{
			ID:         "evt_4",
			Type:       "purchase",
			Timestamp:  time.Now(),
			UserID:     "user_2",
			Properties: map[string]interface{}{"amount": 99.99},
		},
	}

	for _, event := range events {
		agg.ProcessEvent(event)
	}

	metrics := agg.GetGlobalMetrics()

	// Vérifications
	tests := []struct {
		name     string
		key      string
		expected int64
	}{
		{"total_events", "total_events", 4},
		{"pageviews", "pageviews", 2},
		{"clicks", "clicks", 1},
		{"purchases", "purchases", 1},
		{"unique_users", "unique_users", 2},
	}

	for _, tt := range tests {
		if val, ok := metrics[tt.key]; !ok || val.Count != tt.expected {
			actual := int64(0)
			if ok {
				actual = val.Count
			}
			t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, actual)
		}
	}
}

// TestAggregatorPurchaseRevenue teste le tracking de revenue
func TestAggregatorPurchaseRevenue(t *testing.T) {
	logger := zap.NewNop()
	agg := NewAggregator(1*time.Minute, 10*time.Second, logger)

	// Traiter des achats
	purchases := []float64{99.99, 149.99, 49.99}

	for i, amount := range purchases {
		event := Event{
			ID:        fmt.Sprintf("purchase_%d", i),
			Type:      "purchase",
			Timestamp: time.Now(),
			UserID:    fmt.Sprintf("user_%d", i),
			Properties: map[string]interface{}{
				"amount": amount,
			},
		}
		agg.ProcessEvent(event)
	}

	metrics := agg.GetGlobalMetrics()

	revenue := metrics["revenue"]
	expectedRevenue := 299.97

	if math.Abs(revenue.Value-expectedRevenue) > 1e-6 {
		t.Errorf("Expected revenue %.2f, got %.2f", expectedRevenue, revenue.Value)
	}

	if revenue.Count != 3 {
		t.Errorf("Expected 3 purchases, got %d", revenue.Count)
	}
}

// TestAggregatorConcurrency teste la concurrence
func TestAggregatorConcurrency(t *testing.T) {
	logger := zap.NewNop()
	agg := NewAggregator(1*time.Minute, 10*time.Second, logger)

	// Lancer 10 goroutines concurrentes
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(workerID int) {
			// Chaque worker traite 100 événements
			for j := 0; j < 100; j++ {
				event := Event{
					ID:        fmt.Sprintf("evt_%d_%d", workerID, j),
					Type:      "pageview",
					Timestamp: time.Now(),
					UserID:    fmt.Sprintf("user_%d", workerID),
					Properties: map[string]interface{}{
						"page": fmt.Sprintf("/page_%d", j),
					},
				}
				agg.ProcessEvent(event)
			}
			done <- true
		}(i)
	}

	// Attendre tous les workers
	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := agg.GetGlobalMetrics()

	// 10 workers × 100 événements = 1000 événements
	totalMetric, ok := metrics["total_events"]
	if !ok {
		t.Fatal("Expected total_events metric to exist")
	}
	totalEvents := totalMetric.Count
	if totalEvents != 1000 {
		t.Errorf("Expected 1000 events, got %d", totalEvents)
	}

	// 10 utilisateurs uniques
	uniqueMetric, ok := metrics["unique_users"]
	if !ok {
		t.Fatal("Expected unique_users metric to exist")
	}
	uniqueUsers := uniqueMetric.Count
	if uniqueUsers != 10 {
		t.Errorf("Expected 10 unique users, got %d", uniqueUsers)
	}
}

// TestAggregatorStats teste les statistiques
func TestAggregatorStats(t *testing.T) {
	logger := zap.NewNop()
	agg := NewAggregator(1*time.Minute, 10*time.Second, logger)

	// Traiter quelques événements
	for i := 0; i < 5; i++ {
		event := Event{
			ID:         fmt.Sprintf("evt_%d", i),
			Type:       "pageview",
			Timestamp:  time.Now(),
			UserID:     fmt.Sprintf("user_%d", i),
			Properties: map[string]interface{}{},
		}
		agg.ProcessEvent(event)
	}

	stats := agg.GetStats()

	// Vérifier les stats
	if stats["total_events"].(float64) != 5 {
		t.Errorf("Expected 5 total events")
	}

	if stats["metrics_count"].(int) < 1 {
		t.Errorf("Expected at least 1 metric")
	}
}

// BenchmarkMetricIncrement benchmark l'incrémentation
func BenchmarkMetricIncrement(b *testing.B) {
	metric := NewMetric("bench", MetricTypeCounter)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metric.Increment()
	}
}

// BenchmarkAggregatorProcessEvent benchmark le traitement d'événement
func BenchmarkAggregatorProcessEvent(b *testing.B) {
	logger := zap.NewNop()
	agg := NewAggregator(1*time.Minute, 10*time.Second, logger)

	event := Event{
		ID:         "evt_1",
		Type:       "pageview",
		Timestamp:  time.Now(),
		UserID:     "user_1",
		Properties: map[string]interface{}{"page": "/home"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.ProcessEvent(event)
	}
}
