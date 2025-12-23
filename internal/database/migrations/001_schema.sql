-- Schema for AI CLI Server
-- One client = one provider

CREATE TABLE IF NOT EXISTS clients (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  api_key_hash TEXT NOT NULL UNIQUE,
  provider TEXT NOT NULL DEFAULT 'copilot',
  allowed_models TEXT NOT NULL DEFAULT '["*"]',
  default_model TEXT DEFAULT '',
  rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME,
  is_active BOOLEAN DEFAULT TRUE,
  metadata TEXT
);

CREATE TABLE IF NOT EXISTS usage_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_id INTEGER NOT NULL,
  session_id TEXT,
  timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt TEXT,
  prompt_tokens INTEGER,
  completion_tokens INTEGER,
  total_tokens INTEGER,
  cost REAL,
  response_time_ms INTEGER,
  response_status INTEGER,
  error_message TEXT,
  FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rate_limit_buckets (
  client_id INTEGER NOT NULL,
  window_start DATETIME NOT NULL,
  request_count INTEGER DEFAULT 0,
  PRIMARY KEY (client_id, window_start),
  FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_usage_logs_client_id ON usage_logs(client_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_timestamp ON usage_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_logs_session_id ON usage_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_rate_limit_window ON rate_limit_buckets(window_start);
CREATE INDEX IF NOT EXISTS idx_clients_active ON clients(is_active);
