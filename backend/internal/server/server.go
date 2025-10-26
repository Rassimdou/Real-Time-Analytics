package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Rassimdou/Real-time-Analytics/internal/aggregation"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// struct for server
type Server struct {
	engine     *gin.Engine
	httpServer *http.Server
	logger     *zap.Logger
	eventQueue chan Event
	aggregator *aggregation.Aggregator
}

// Event represents an analytics event
type Event struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type" binding:"required"`
	Timestamp  time.Time              `json:"timestamp"`
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	Properties map[string]interface{} `json:"properties"`
}

type ErrorResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// NewServer creates a new server instance
func NewServer(addr string, logger *zap.Logger, eventQueue chan Event, aggregator *aggregation.Aggregator, mode string) *Server {
	// set GIN mode (debug, release, test)
	gin.SetMode(mode)

	s := &Server{
		engine:     gin.New(),
		logger:     logger,
		eventQueue: eventQueue,
		aggregator: aggregator,
	}
	s.setupMiddleware()
	s.setupRoutes()

	//create http server
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return s
}

// setup middleware
func (s *Server) setupMiddleware() {
	//Recover middleware (panic recovery)
	s.engine.Use(gin.Recovery())

	//Logging middleware with zap
	s.engine.Use(s.zapLoggerMiddleware())

	//CORS middleware
	s.engine.Use(s.corsMiddleware())

	//Request ID middleware
	s.engine.Use(s.requestIDMiddleware())
}

// setup routes configures HTTP routes
func (s *Server) setupRoutes() {

	//Health check endpoint
	s.engine.GET("/health", s.handleHealth)
	s.engine.GET("/ready", s.handleReady)

	//v1
	v1 := s.engine.Group("/api/v1")
	{

		//EVENT ingestion
		v1.POST("/events", s.handleEvent)
		v1.POST("/events/batch", s.handleBatchEvents)

		//Metrics (placeholder for future)
		v1.GET("/metrics", s.handleGetAllMetrics)
		v1.GET("/metrics/:name", s.handleGetMetricByName)
		v1.GET("/stats", s.handleGetStats)
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server",
		zap.String("addr", s.httpServer.Addr),
		zap.String("mode", gin.Mode()),
	)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not start server: %w", err)
	}
	return nil
}

// shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().UTC(),
	})
}

// handleReady handles readiness check requests
func (s *Server) handleReady(c *gin.Context) {
	//TODO: Check database connection, redis ....
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"time":   time.Now().UTC(),
		"checks": gin.H{
			"database": "ok",
			"redis":    "ok",
			"queue":    "ok",
		},
	})
}

// handleEvent handles single event ingestion
func (s *Server) handleEvent(c *gin.Context) {
	var event Event

	// Bind and validate JSON
	if err := c.ShouldBindJSON(&event); err != nil {
		s.logger.Error("failed to bind event", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   true,
			Message: "invalid event data" + err.Error(),
		})
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}

	// Try to send to queue (non-blocking)
	select {
	case s.eventQueue <- event:
		s.logger.Debug("event queued",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.Type),
			zap.String("user_id", event.UserID),
		)

		c.JSON(http.StatusAccepted, SuccessResponse{
			Status:  "accepted",
			Message: "event queued for processing",
			Data: gin.H{
				"event_id": event.ID,
				"type":     event.Type,
			},
		})
	default:
		// Queue is full
		s.logger.Warn("event queue full, rejecting event",
			zap.String("event_type", event.Type),
		)
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   true,
			Message: "queue full, try again later",
		})
	}
}

// handleBatchEvents handles batch event ingestion
func (s *Server) handleBatchEvents(c *gin.Context) {
	var events []Event

	// Bind and validate JSON
	if err := c.ShouldBindJSON(&events); err != nil {
		s.logger.Error("failed to bind batch events", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   true,
			Message: "invalid batch data: " + err.Error(),
		})
		return
	}

	// Validate batch size
	if len(events) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   true,
			Message: "empty batch",
		})
		return
	}

	if len(events) > 1000 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   true,
			Message: "batch size exceeds limit of 1000",
		})
		return
	}

	accepted := 0
	rejected := 0
	now := time.Now().UTC()

	//Queue all events
	for i := range events {
		event := &events[i]

		//set timestamp if missing
		if event.Timestamp.IsZero() {
			event.Timestamp = now
		}

		//Generate ID if missing
		if event.ID == "" {
			event.ID = fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), i)

		}

		//Validate event type
		if event.Type == "" {
			rejected++
			continue
		}

		//Try to send to queue (non-blocking)
		select {
		case s.eventQueue <- *event:
			accepted++
		default:
			//full queue
			rejected++
		}
	}

	s.logger.Debug("batch events processed",
		zap.Int("total", len(events)),
		zap.Int("accepted", accepted),
		zap.Int("rejected", rejected),
	)

	c.JSON(http.StatusAccepted, SuccessResponse{
		Status:  "accepted",
		Message: fmt.Sprintf("batch processed: %d events", len(events)),
		Data: gin.H{
			"total":    len(events),
			"accepted": accepted,
			"rejected": rejected,
		},
	})
}

// handleGetMetrics handles metrics retrieval (placeholder)
// handleGetAllMetrics retourne TOUTES les métriques
func (s *Server) handleGetAllMetrics(c *gin.Context) {
	if s.aggregator == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   true,
			Message: "aggregator not initialized",
		})
		return
	}

	// Récupérer toutes les métriques depuis l'aggregator
	metrics := s.aggregator.GetGlobalMetrics()

	if len(metrics) == 0 {
		c.JSON(http.StatusOK, SuccessResponse{
			Status:  "success",
			Message: "no metrics yet",
			Data: gin.H{
				"metrics": []string{},
			},
		})
		return
	}

	s.logger.Debug("returning all metrics",
		zap.Int("count", len(metrics)),
	)

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: fmt.Sprintf("retrieved %d metrics", len(metrics)),
		Data:    metrics,
	})
}

// handleGetMetricByName retourne une métrique spécifique
func (s *Server) handleGetMetricByName(c *gin.Context) {
	if s.aggregator == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   true,
			Message: "aggregator not initialized",
		})
		return
	}

	metricName := c.Param("name")

	// Récupérer toutes les métriques
	allMetrics := s.aggregator.GetGlobalMetrics()

	// Chercher la métrique demandée
	metric, exists := allMetrics[metricName]
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   true,
			Message: fmt.Sprintf("metric '%s' not found", metricName),
		})
		return
	}

	s.logger.Debug("returning metric",
		zap.String("metric_name", metricName),
		zap.Float64("value", metric.Value),
		zap.Int64("count", metric.Count),
	)

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: fmt.Sprintf("metric '%s' found", metricName),
		Data: gin.H{
			"name":      metric.Name,
			"type":      metric.Type,
			"value":     metric.Value,
			"count":     metric.Count,
			"timestamp": metric.Timestamp,
		},
	})
}

// handleGetStats retourne les statistiques de l'aggregator
func (s *Server) handleGetStats(c *gin.Context) {
	if s.aggregator == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   true,
			Message: "aggregator not initialized",
		})
		return
	}

	// Récupérer les stats
	stats := s.aggregator.GetStats()

	s.logger.Info("returning aggregator stats",
		zap.Any("stats", stats),
	)

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "aggregator statistics",
		Data:    stats,
	})
}

// zapLoggerMiddleware creates a Gin middleware using zap logger
func (s *Server) zapLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		//Process request
		c.Next()

		// Log after request
		latency := time.Since(start)

		fields := []zap.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", raw),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
		}
		if len(c.Errors) > 0 {
			// Append errors to log
			s.logger.Error("request completed with errors", fields...)
		} else {
			s.logger.Info("request completed", fields...)
		}
	}
}

// corsMiddleware handles CORS settings
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

// requestIDMiddleware adds a unique request ID to each request
func (s *Server) requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		c.Set("RequestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}
