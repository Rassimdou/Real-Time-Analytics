package aggregation

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Event représente un événement à agréger
type Event struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	Properties map[string]interface{} `json:"properties"`
}

// Aggregator agrège les événements en métriques
type Aggregator struct {
	// Métriques globales (depuis le début)
	globalMetrics *MetricsSnapshot

	// Fenêtres de temps (1 minute)
	windowManager *WindowManager

	// Configuration
	windowDuration time.Duration
	flushInterval  time.Duration

	// Callbacks
	onWindowClosed func(*TimeWindow)

	// Logger
	logger *zap.Logger

	// Synchronisation
	mu sync.RWMutex
}

// NewAggregator crée un nouvel agrégateur
func NewAggregator(windowDuration, flushInterval time.Duration, logger *zap.Logger) *Aggregator {
	return &Aggregator{
		globalMetrics:  NewMetricsSnapshot(),
		windowManager:  NewWindowManager(windowDuration),
		windowDuration: windowDuration,
		flushInterval:  flushInterval,
		logger:         logger,
	}
}

// SetWindowClosedCallback définit un callback appelé quand une fenêtre se ferme
func (a *Aggregator) SetWindowClosedCallback(callback func(*TimeWindow)) {
	a.onWindowClosed = callback
}

// Start démarre l'agrégateur
func (a *Aggregator) Start(ctx context.Context) {
	a.logger.Info("aggregator started",
		zap.Duration("window_duration", a.windowDuration),
		zap.Duration("flush_interval", a.flushInterval),
	)

	ticker := time.NewTicker(a.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("aggregator stopping")
			return

		case <-ticker.C:
			a.flushExpiredWindows()
			a.cleanup()
		}
	}
}

// ProcessEvent traite un événement et met à jour les métriques
func (a *Aggregator) ProcessEvent(event Event) {
	now := time.Now()

	// Mettre à jour métriques globales
	a.updateGlobalMetrics(event)

	// Mettre à jour fenêtre de temps
	window := a.windowManager.GetOrCreateWindow(event.Timestamp)
	a.updateWindowMetrics(window, event)

	a.logger.Debug("event processed",
		zap.String("event_type", event.Type),
		zap.String("user_id", event.UserID),
		zap.Duration("processing_time", time.Since(now)),
	)
}

// updateGlobalMetrics met à jour les métriques globales
func (a *Aggregator) updateGlobalMetrics(event Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Compteur total d'événements
	totalEvents := a.globalMetrics.GetMetric("total_events", MetricTypeCounter)
	totalEvents.Increment()

	// Compteur par type d'événement
	eventTypeKey := "events_by_type:" + event.Type
	eventTypeMetric := a.globalMetrics.GetMetric(eventTypeKey, MetricTypeCounter)
	eventTypeMetric.Increment()

	// Utilisateurs uniques
	uniqueUsers := a.globalMetrics.GetMetric("unique_users", MetricTypeSet)
	if event.UserID != "" {
		uniqueUsers.AddUnique(event.UserID)
	}

	// Sessions uniques
	uniqueSessions := a.globalMetrics.GetMetric("unique_sessions", MetricTypeSet)
	if event.SessionID != "" {
		uniqueSessions.AddUnique(event.SessionID)
	}

	// Métriques spécifiques par type d'événement
	switch event.Type {
	case "pageview":
		a.processPageviewMetrics(event)
	case "click":
		a.processClickMetrics(event)
	case "purchase":
		a.processPurchaseMetrics(event)
	}
}

// processPageviewMetrics traite les métriques de pageview
func (a *Aggregator) processPageviewMetrics(event Event) {
	// Compteur de pageviews
	pageviews := a.globalMetrics.GetMetric("pageviews", MetricTypeCounter)
	pageviews.Increment()

	// Pages uniques visitées
	if page, ok := event.Properties["page"].(string); ok {
		uniquePages := a.globalMetrics.GetMetric("unique_pages", MetricTypeSet)
		uniquePages.AddUnique(page)

		// Compteur par page
		pageKey := "page_views:" + page
		pageMetric := a.globalMetrics.GetMetric(pageKey, MetricTypeCounter)
		pageMetric.Increment()
	}
}

// processClickMetrics traite les métriques de click
func (a *Aggregator) processClickMetrics(event Event) {
	// Compteur de clicks
	clicks := a.globalMetrics.GetMetric("clicks", MetricTypeCounter)
	clicks.Increment()

	// Clicks par élément
	if element, ok := event.Properties["element"].(string); ok {
		elementKey := "clicks:" + element
		elementMetric := a.globalMetrics.GetMetric(elementKey, MetricTypeCounter)
		elementMetric.Increment()
	}
}

// processPurchaseMetrics traite les métriques d'achat
func (a *Aggregator) processPurchaseMetrics(event Event) {
	// Compteur d'achats
	purchases := a.globalMetrics.GetMetric("purchases", MetricTypeCounter)
	purchases.Increment()

	// Revenue total
	if amount, ok := event.Properties["amount"].(float64); ok {
		revenue := a.globalMetrics.GetMetric("revenue", MetricTypeCounter)
		revenue.IncrementBy(amount)

		// Histogramme des montants
		revenueHist := a.globalMetrics.GetMetric("revenue_histogram", MetricTypeHistogram)
		revenueHist.Observe(amount)
	}
}

// updateWindowMetrics met à jour les métriques d'une fenêtre
func (a *Aggregator) updateWindowMetrics(window *TimeWindow, event Event) {
	// Événements dans cette fenêtre
	windowEvents := window.Metrics.GetMetric("events", MetricTypeCounter)
	windowEvents.Increment()

	// Par type
	eventTypeKey := "events:" + event.Type
	eventTypeMetric := window.Metrics.GetMetric(eventTypeKey, MetricTypeCounter)
	eventTypeMetric.Increment()

	// Utilisateurs actifs dans la fenêtre
	activeUsers := window.Metrics.GetMetric("active_users", MetricTypeSet)
	if event.UserID != "" {
		activeUsers.AddUnique(event.UserID)
	}
}

// flushExpiredWindows ferme et traite les fenêtres expirées
func (a *Aggregator) flushExpiredWindows() {
	now := time.Now()
	closedWindows := a.windowManager.CloseExpiredWindows(now)

	if len(closedWindows) > 0 {
		a.logger.Info("flushing expired windows",
			zap.Int("count", len(closedWindows)),
		)

		for _, window := range closedWindows {
			a.logger.Debug("window closed",
				zap.Time("start", window.StartTime),
				zap.Time("end", window.EndTime),
				zap.Int("metrics_count", len(window.Metrics.Metrics)),
			)

			// Appeler callback si défini
			if a.onWindowClosed != nil {
				a.onWindowClosed(window)
			}
		}
	}
}

// cleanup nettoie les anciennes fenêtres
func (a *Aggregator) cleanup() {
	// Garder les fenêtres fermées pendant 5 minutes
	a.windowManager.Cleanup(5 * time.Minute)
}

// GetGlobalMetrics retourne les métriques globales
func (a *Aggregator) GetGlobalMetrics() map[string]*Metric {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.globalMetrics.GetAllMetrics()
}

// GetGlobalMetricValue retourne la valeur d'une métrique globale
func (a *Aggregator) GetGlobalMetricValue(name string) (float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.globalMetrics.GetMetricValue(name)
}

// GetActiveWindows retourne les fenêtres actives
func (a *Aggregator) GetActiveWindows() []*TimeWindow {
	return a.windowManager.GetActiveWindows()
}

// GetStats retourne les statistiques de l'agrégateur
func (a *Aggregator) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalEvents, _ := a.globalMetrics.GetMetricValue("total_events")
	uniqueUsers, _ := a.globalMetrics.GetMetricValue("unique_users")
	uniqueSessions, _ := a.globalMetrics.GetMetricValue("unique_sessions")

	activeWindows := a.windowManager.GetActiveWindows()

	return map[string]interface{}{
		"total_events":    totalEvents,
		"unique_users":    uniqueUsers,
		"unique_sessions": uniqueSessions,
		"active_windows":  len(activeWindows),
		"metrics_count":   len(a.globalMetrics.Metrics),
		"uptime":          time.Since(a.globalMetrics.Timestamp),
	}
}

// Reset réinitialise toutes les métriques (pour tests)
func (a *Aggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.globalMetrics.Reset()
	a.windowManager = NewWindowManager(a.windowDuration)

	a.logger.Info("aggregator reset")
}
