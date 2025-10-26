CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE TABLE IF NOT EXISTS events (
    id TEXT NOT NULL,
    time TIMESTAMPTZ NOT NULL,
    type TEXT NOT NULL,
    user_id TEXT,
    session_id TEXT,
    properties JSONB,
    
    -- Métadonnées
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Créer une hypertable TimescaleDB
-- (Si la table n'est pas déjà une hypertable)
SELECT create_hypertable('events', 'time', if_not_exists => TRUE);

-- Indexes pour les requêtes rapides
CREATE INDEX IF NOT EXISTS idx_events_type_time 
    ON events (type, time DESC);

CREATE INDEX IF NOT EXISTS idx_events_user_time 
    ON events (user_id, time DESC);

CREATE INDEX IF NOT EXISTS idx_events_session_time 
    ON events (session_id, time DESC);

-- Index JSONB pour les properties
CREATE INDEX IF NOT EXISTS idx_events_properties 
    ON events USING GIN (properties);

-- ============================================
-- Table des métriques agrégées (1 minute)
-- ============================================

CREATE TABLE IF NOT EXISTS metrics_1m (
    time TIMESTAMPTZ NOT NULL,
    metric_name TEXT NOT NULL,
    metric_type TEXT NOT NULL,  -- counter, gauge, histogram, set
    
    -- Valeurs
    value DOUBLE PRECISION,
    count BIGINT,
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    
    -- JSON pour les données complexes
    data JSONB,
    
    -- Métadonnées
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Créer une hypertable
SELECT create_hypertable('metrics_1m', 'time', if_not_exists => TRUE);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_metrics_1m_name_time 
    ON metrics_1m (metric_name, time DESC);

CREATE INDEX IF NOT EXISTS idx_metrics_1m_type 
    ON metrics_1m (metric_type, time DESC);

-- ============================================
-- Table des métriques agrégées (1 heure)
-- ============================================

CREATE TABLE IF NOT EXISTS metrics_1h (
    time TIMESTAMPTZ NOT NULL,
    metric_name TEXT NOT NULL,
    
    value DOUBLE PRECISION,
    count BIGINT,
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('metrics_1h', 'time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_metrics_1h_name_time 
    ON metrics_1h (metric_name, time DESC);

-- ============================================
-- Table des métriques agrégées (1 jour)
-- ============================================

CREATE TABLE IF NOT EXISTS metrics_1d (
    time TIMESTAMPTZ NOT NULL,
    metric_name TEXT NOT NULL,
    
    value DOUBLE PRECISION,
    count BIGINT,
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('metrics_1d', 'time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_metrics_1d_name_time 
    ON metrics_1d (metric_name, time DESC);

-- ============================================
-- Table des fenêtres de temps fermées
-- ============================================

CREATE TABLE IF NOT EXISTS closed_windows (
    id BIGSERIAL PRIMARY KEY,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    window_duration INTERVAL NOT NULL,
    
    -- Résumé des métriques
    total_events BIGINT,
    event_types JSONB,  -- {"pageview": 100, "click": 50, ...}
    unique_users BIGINT,
    unique_sessions BIGINT,
    
    -- Toutes les métriques (pour reconstruction)
    metrics_data JSONB,
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index
CREATE INDEX IF NOT EXISTS idx_closed_windows_time 
    ON closed_windows (start_time DESC, end_time DESC);

CREATE INDEX IF NOT EXISTS idx_closed_windows_created 
    ON closed_windows (created_at DESC);

-- ============================================
-- Vues pour les requêtes communes
-- ============================================

-- Vue: Événements du dernier jour
CREATE OR REPLACE VIEW v_events_last_day AS
SELECT * FROM events 
WHERE time > NOW() - INTERVAL '1 day'
ORDER BY time DESC;

-- Vue: Métriques du dernier jour (1m)
CREATE OR REPLACE VIEW v_metrics_last_day AS
SELECT * FROM metrics_1m 
WHERE time > NOW() - INTERVAL '1 day'
ORDER BY time DESC;

-- Vue: Événements par type (dernière heure)
CREATE OR REPLACE VIEW v_events_by_type_1h AS
SELECT 
    type,
    COUNT(*) as count,
    COUNT(DISTINCT user_id) as unique_users,
    COUNT(DISTINCT session_id) as unique_sessions,
    MIN(time) as earliest,
    MAX(time) as latest
FROM events
WHERE time > NOW() - INTERVAL '1 hour'
GROUP BY type
ORDER BY count DESC;

-- Vue: Top pages visitées
CREATE OR REPLACE VIEW v_top_pages_1h AS
SELECT 
    properties->>'page' as page,
    COUNT(*) as views,
    COUNT(DISTINCT user_id) as unique_users
FROM events
WHERE type = 'pageview' 
    AND time > NOW() - INTERVAL '1 hour'
    AND properties->>'page' IS NOT NULL
GROUP BY properties->>'page'
ORDER BY views DESC
LIMIT 50;

-- Vue: Revenue par jour
CREATE OR REPLACE VIEW v_revenue_daily AS
SELECT 
    DATE(time) as date,
    COUNT(*) as purchases,
    SUM((properties->>'amount')::DOUBLE PRECISION) as total_revenue,
    AVG((properties->>'amount')::DOUBLE PRECISION) as avg_revenue,
    COUNT(DISTINCT user_id) as unique_buyers
FROM events
WHERE type = 'purchase' 
    AND properties->>'amount' IS NOT NULL
GROUP BY DATE(time)
ORDER BY date DESC;