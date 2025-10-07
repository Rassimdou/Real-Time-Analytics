package aggregation

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Event represente un evenement a agreger
type Event struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type`
	Timestamp  time.Time              `json:"timestamp"`
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	Properties map[string]interface{} `json:"properties"`
}

// Aggregator agrege les evenement en metriques
type Aggregator struct {
	globalMetrics  *MetricSnapshot //metriques globales (depuis le debut)
	windowManager  *WindowManager  // Fenetres de temps (1min)
	windowDuration time.Time
	FlushInterval  time.Duration
	onWindowClosed func(*TimeWindow) //callbacks
	logger         *zap.logger
	mu             sunc.RWMutex //synchronisation
}

func NewAggregator(windowDuration, flushInterval time.Duration, logger *zap.Logger) *Aggregator {
	return &Aggregator{
		globalMetrics:  NewMetricSnapshot(),
		windowManager:  NewWindowManager(windowDuration),
		windowDuration: windowDuration,
		FlushInterval:  flushInterval,
		logger:         logger,
	}
}

// définit un callback appelé quand une fenêtre se ferme
func (a *Aggregator) SetWindoClosedCallback(callback func(*TimeWindow)) {
	a.onWindowClosed = callback
}

func (a *Aggregator) Start(ctx context.Context) {
	a.logger.Info("Starting Aggregator",
		zap.Duration("Window duration", a.windowDuration),
		zap.Duration("Flush Interval", a.FlushInterval),
	)

	ticker := time.NewTicker(a.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Aggregator stopping")
			return
		case <-ticker.C:
			a.flushExpiredWindows()
			a.cleanup()

		}
	}

}

// traite un evenement et met a jour les metriques
func (a *Aggregator) ProcessEvent(event Event) {
	now := time.Now()

	//update global metrics
	a.updateGlobalMetrics(event)

	window := a.windowManager.GetOrCreateWindow(event.Timestamp)
	a.updateWindowMetrics(window, event)

	a.logger.Debug("event Processed",
		zap.String("event_type", event.Type),
		zap.String("user_id", event.UserID),
		zap.Duration("processing_time", time.Since(now)),
	)
}

func (a *Aggregator) updateGlobalMetrics(event Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	//compteur total devenemnt
	totalEvents := a.globalMetrics.GetMetric("total_events", MetricTypeCounter)
	totalEvents.Increment()
}
