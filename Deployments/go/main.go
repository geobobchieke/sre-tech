package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Transaction struct {
	ID        string    `json:"id" db:"id"`
	Value     float64   `json:"value" db:"value"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type TransactionRequest struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type App struct {
	db               *sql.DB
	httpDuration     *prometheus.HistogramVec
	httpRequests     *prometheus.CounterVec
	txnCounter       *prometheus.CounterVec
	txnValueSum      prometheus.Gauge
	txnErrorCounter  prometheus.Counter
	httpRequestSize  *prometheus.HistogramVec
	httpResponseSize *prometheus.HistogramVec
}

func main() {
	app := &App{}
	app.initMetrics()
	app.initDB()
	app.setupRoutes()
}

func (a *App) initMetrics() {
	// Initialize metrics
	a.httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "Duration of HTTP requests",
		},
		[]string{"path", "method", "status_code"},
	)

	a.httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status_code"},
	)

	a.txnCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transactions_total",
			Help: "Total number of transactions",
		},
		[]string{"status"},
	)

	a.txnValueSum = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "transactions_value_sum",
		Help: "Sum of all transaction values processed.",
	})

	a.txnErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "transactions_errors_total",
		Help: "Number of failed transaction creations.",
	})

	a.httpRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "Size of HTTP requests",
			Buckets: prometheus.ExponentialBuckets(100, 10, 5), // 100B, 1KB, 10KB, 100KB, 1MB
		},
		[]string{"path", "method"},
	)

	a.httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP responses",
			Buckets: prometheus.ExponentialBuckets(100, 10, 5),
		},
		[]string{"path", "method", "status_code"},
	)

	// Register all metrics, plus Go runtime and process collectors, but avoid duplicates
	registerMetric(a.httpDuration)
	registerMetric(a.httpRequests)
	registerMetric(a.txnCounter)
	registerMetric(a.txnValueSum)
	registerMetric(a.txnErrorCounter)
	registerMetric(a.httpRequestSize)
	registerMetric(a.httpResponseSize)
	registerMetric(prometheus.NewGoCollector())
	registerMetric(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
}

// Helper function to register a metric if it is not already registered
func registerMetric(metric prometheus.Collector) {
	if err := prometheus.Register(metric); err != nil {
		// Log the error, but continue execution
		log.Printf("Metrics registration skipped: %v", err)
	}
}

func (a *App) initDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost/transactions?sslmode=disable"
	}

	var err error
	a.db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Create table if not exists
	createTable := `
	CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		value DECIMAL(15,2) NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		status VARCHAR(50) DEFAULT 'completed',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := a.db.Exec(createTable); err != nil {
		log.Fatal("Failed to create table:", err)
	}

	a.collectDBStats()
}

func (a *App) collectDBStats() {
	dbMaxOpenConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_max_open_connections",
		Help: "Maximum number of open connections to the database.",
	})
	dbOpenConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_open_connections",
		Help: "The number of established connections both in use and idle.",
	})
	dbInUseConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_in_use_connections",
		Help: "The number of connections currently in use.",
	})
	dbIdleConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_idle_connections",
		Help: "The number of idle connections.",
	})

	// Register DB metrics
	registerMetric(dbMaxOpenConns)
	registerMetric(dbOpenConns)
	registerMetric(dbInUseConns)
	registerMetric(dbIdleConns)

	go func() {
		for {
			stats := a.db.Stats()
			dbMaxOpenConns.Set(float64(stats.MaxOpenConnections))
			dbOpenConns.Set(float64(stats.OpenConnections))
			dbInUseConns.Set(float64(stats.InUse))
			dbIdleConns.Set(float64(stats.Idle))
			time.Sleep(10 * time.Second)
		}
	}()
}

func (a *App) setupRoutes() {
	r := mux.NewRouter()

	// Add metrics middleware
	r.Use(a.metricsMiddleware)

	r.HandleFunc("/transactions", a.createTransaction).Methods("POST")
	r.HandleFunc("/transactions", a.listTransactions).Methods("GET")
	r.HandleFunc("/transactions/{id}", a.getTransaction).Methods("GET")
	r.HandleFunc("/health", a.healthCheck).Methods("GET")
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func (a *App) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200, size: 0}

		if r.ContentLength > 0 {
			a.httpRequestSize.WithLabelValues(r.URL.Path, r.Method).Observe(float64(r.ContentLength))
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(wrapped.statusCode)

		a.httpDuration.WithLabelValues(r.URL.Path, r.Method, statusCode).Observe(duration)
		a.httpRequests.WithLabelValues(r.URL.Path, r.Method, statusCode).Inc()
		a.httpResponseSize.WithLabelValues(r.URL.Path, r.Method, statusCode).Observe(float64(wrapped.size))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

func (a *App) createTransaction(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.txnErrorCounter.Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Value <= 0 {
		a.txnErrorCounter.Inc()
		http.Error(w, "Transaction value must be positive", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO transactions (value, timestamp, status, created_at)
		VALUES ($1, $2, 'completed', CURRENT_TIMESTAMP)
		RETURNING id, value, timestamp, status, created_at`

	var txn Transaction
	err := a.db.QueryRow(query, req.Value, req.Timestamp).Scan(
		&txn.ID, &txn.Value, &txn.Timestamp, &txn.Status, &txn.CreatedAt)

	if err != nil {
		log.Printf("Database error: %v", err)
		a.txnErrorCounter.Inc()
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	a.txnCounter.WithLabelValues("completed").Inc()
	a.txnValueSum.Add(req.Value)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(txn)
}

func (a *App) listTransactions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	query := `
		SELECT id, value, timestamp, status, created_at 
		FROM transactions 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`

	rows, err := a.db.Query(query, limit, offset)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var txn Transaction
		err := rows.Scan(&txn.ID, &txn.Value, &txn.Timestamp, &txn.Status, &txn.CreatedAt)
		if err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}
		transactions = append(transactions, txn)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

func (a *App) getTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	query := `
		SELECT id, value, timestamp, status, created_at 
		FROM transactions 
		WHERE id = $1`

	var txn Transaction
	err := a.db.QueryRow(query, id).Scan(
		&txn.ID, &txn.Value, &txn.Timestamp, &txn.Status, &txn.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txn)
}

func (a *App) healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  "database connection failed",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}
