# Section 18: Production Readiness

## 🏭 Preparing for Production Deployment

Welcome to Section 18! This section focuses on ensuring your blockchain application is truly production-ready. You'll learn about security hardening, performance optimization, monitoring, scalability, and disaster recovery to prepare your application for real-world deployment.

---

## 📚 Learning Objectives

By the end of this section, you will be able to:

✅ **Implement Security Hardening**: Apply production-grade security measures  
✅ **Optimize Performance**: Tune your application for production workloads  
✅ **Set Up Monitoring**: Implement comprehensive monitoring and alerting  
✅ **Plan for Scalability**: Design systems that can grow with demand  
✅ **Ensure High Availability**: Implement fault tolerance and redundancy  
✅ **Create Disaster Recovery**: Plan for and implement backup and recovery  
✅ **Deploy to Production**: Safely deploy and manage production systems  

---

## 🛠️ Prerequisites

Before starting this section, ensure you have:

- **Phase 1**: Basic blockchain implementation (all sections)
- **Phase 2**: Advanced features and APIs (all sections)
- **Phase 3**: User experience and interface development (all sections)
- **Section 16**: Testing and quality assurance
- **Section 17**: Build system and deployment
- **Production Knowledge**: Understanding of production deployment concepts
- **Security Awareness**: Basic knowledge of security best practices

---

## 📋 Section Overview

### **What You'll Build**

In this section, you'll create a production-ready blockchain system that includes:

- **Security Hardening**: Comprehensive security measures and best practices
- **Performance Optimization**: Tuned for production workloads and scalability
- **Monitoring & Alerting**: Real-time monitoring and automated alerting
- **High Availability**: Fault tolerance and redundancy implementation
- **Disaster Recovery**: Backup strategies and recovery procedures
- **Production Deployment**: Safe deployment and management procedures
- **Compliance & Governance**: Regulatory compliance and governance frameworks

### **Key Technologies**

- **Security Tools**: OWASP guidelines, security scanning, penetration testing
- **Monitoring**: Prometheus, Grafana, ELK Stack, application metrics
- **Performance**: Load testing, profiling, optimization techniques
- **High Availability**: Load balancing, clustering, failover mechanisms
- **Backup & Recovery**: Automated backup systems, disaster recovery plans
- **Compliance**: GDPR, SOC 2, ISO 27001, regulatory frameworks
- **Governance**: Access controls, audit logging, policy enforcement

---

## 🎯 Core Concepts

### **1. Security Hardening**

#### **Application Security**
```go
// internal/security/security.go
package security

import (
    "crypto/rand"
    "crypto/tls"
    "net/http"
    "time"
    "golang.org/x/crypto/bcrypt"
    "github.com/golang-jwt/jwt/v5"
)

// SecurityConfig holds security configuration
type SecurityConfig struct {
    JWTSecret        string
    BCryptCost       int
    SessionTimeout   time.Duration
    MaxLoginAttempts int
    RateLimitPerMin  int
}

// SecureHeaders adds security headers to HTTP responses
func SecureHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Security headers
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        
        next.ServeHTTP(w, r)
    })
}

// RateLimiter implements rate limiting middleware
type RateLimiter struct {
    requests map[string][]time.Time
    limit    int
    window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
    return &RateLimiter{
        requests: make(map[string][]time.Time),
        limit:    limit,
        window:   window,
    }
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        clientIP := getClientIP(r)
        now := time.Now()
        
        // Clean old requests
        if requests, exists := rl.requests[clientIP]; exists {
            var validRequests []time.Time
            for _, reqTime := range requests {
                if now.Sub(reqTime) < rl.window {
                    validRequests = append(validRequests, reqTime)
                }
            }
            rl.requests[clientIP] = validRequests
        }
        
        // Check rate limit
        if len(rl.requests[clientIP]) >= rl.limit {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        // Add current request
        rl.requests[clientIP] = append(rl.requests[clientIP], now)
        
        next.ServeHTTP(w, r)
    })
}

// InputSanitizer sanitizes user input
func SanitizeInput(input string) string {
    // Remove potentially dangerous characters
    sanitized := strings.ReplaceAll(input, "<script>", "")
    sanitized = strings.ReplaceAll(sanitized, "</script>", "")
    sanitized = strings.ReplaceAll(sanitized, "javascript:", "")
    sanitized = strings.ReplaceAll(sanitized, "onload=", "")
    sanitized = strings.ReplaceAll(sanitized, "onerror=", "")
    
    return sanitized
}

// SecurePassword hashes passwords securely
func SecurePassword(password string) (string, error) {
    hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    if err != nil {
        return "", err
    }
    return string(hashedBytes), nil
}

// VerifyPassword verifies a password against its hash
func VerifyPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// GenerateSecureToken generates a secure JWT token
func GenerateSecureToken(userID string, secret string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(time.Hour * 24).Unix(),
        "iat":     time.Now().Unix(),
        "iss":     "blockchain-app",
    })
    
    return token.SignedString([]byte(secret))
}

// ValidateToken validates a JWT token
func ValidateToken(tokenString, secret string) (string, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(secret), nil
    })
    
    if err != nil {
        return "", err
    }
    
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        userID := claims["user_id"].(string)
        return userID, nil
    }
    
    return "", fmt.Errorf("invalid token")
}

// TLSConfig creates a secure TLS configuration
func TLSConfig() *tls.Config {
    return &tls.Config{
        MinVersion:               tls.VersionTLS12,
        CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256},
        PreferServerCipherSuites: true,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
    }
}

// GenerateSecureRandom generates cryptographically secure random bytes
func GenerateSecureRandom(length int) ([]byte, error) {
    bytes := make([]byte, length)
    _, err := rand.Read(bytes)
    return bytes, err
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
    // Check for forwarded headers
    if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
        return strings.Split(ip, ",")[0]
    }
    if ip := r.Header.Get("X-Real-IP"); ip != "" {
        return ip
    }
    if ip := r.Header.Get("X-Client-IP"); ip != "" {
        return ip
    }
    
    // Fall back to remote address
    ip, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return ip
}
```

#### **Database Security**
```go
// internal/database/security.go
package database

import (
    "database/sql"
    "fmt"
    "log"
    "time"
)

// DatabaseSecurity implements database security measures
type DatabaseSecurity struct {
    db *sql.DB
}

// NewDatabaseSecurity creates a new database security instance
func NewDatabaseSecurity(db *sql.DB) *DatabaseSecurity {
    return &DatabaseSecurity{db: db}
}

// EnableSSL enables SSL connections
func (ds *DatabaseSecurity) EnableSSL() error {
    // Configure SSL for database connections
    query := "SET sslmode=require"
    _, err := ds.db.Exec(query)
    if err != nil {
        return fmt.Errorf("failed to enable SSL: %v", err)
    }
    return nil
}

// SetConnectionLimits sets connection limits
func (ds *DatabaseSecurity) SetConnectionLimits() error {
    queries := []string{
        "SET max_connections = 100",
        "SET shared_preload_libraries = 'pg_stat_statements'",
        "SET log_statement = 'all'",
        "SET log_min_duration_statement = 1000",
    }
    
    for _, query := range queries {
        _, err := ds.db.Exec(query)
        if err != nil {
            return fmt.Errorf("failed to set connection limits: %v", err)
        }
    }
    return nil
}

// AuditLogging enables audit logging
func (ds *DatabaseSecurity) AuditLogging() error {
    // Create audit log table
    createTable := `
    CREATE TABLE IF NOT EXISTS audit_log (
        id SERIAL PRIMARY KEY,
        user_id VARCHAR(255),
        action VARCHAR(255),
        table_name VARCHAR(255),
        record_id VARCHAR(255),
        old_values JSONB,
        new_values JSONB,
        ip_address INET,
        user_agent TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )
    `
    
    _, err := ds.db.Exec(createTable)
    if err != nil {
        return fmt.Errorf("failed to create audit log table: %v", err)
    }
    
    return nil
}

// LogAuditEvent logs an audit event
func (ds *DatabaseSecurity) LogAuditEvent(userID, action, tableName, recordID string, oldValues, newValues interface{}, ipAddress, userAgent string) error {
    query := `
    INSERT INTO audit_log (user_id, action, table_name, record_id, old_values, new_values, ip_address, user_agent)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `
    
    _, err := ds.db.Exec(query, userID, action, tableName, recordID, oldValues, newValues, ipAddress, userAgent)
    if err != nil {
        return fmt.Errorf("failed to log audit event: %v", err)
    }
    
    return nil
}

// BackupDatabase creates a secure backup
func (ds *DatabaseSecurity) BackupDatabase(backupPath string) error {
    // Implementation would depend on the database system
    // For PostgreSQL, you might use pg_dump
    log.Printf("Creating database backup to: %s", backupPath)
    
    // Example backup command (would need to be implemented based on your database)
    // cmd := exec.Command("pg_dump", "-h", host, "-U", user, "-d", database, "-f", backupPath)
    // return cmd.Run()
    
    return nil
}
```

### **2. Performance Optimization**

#### **Application Performance**
```go
// internal/performance/optimizer.go
package performance

import (
    "context"
    "runtime"
    "runtime/pprof"
    "sync"
    "time"
)

// PerformanceOptimizer manages application performance
type PerformanceOptimizer struct {
    cache     map[string]interface{}
    cacheMux  sync.RWMutex
    metrics   *Metrics
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer() *PerformanceOptimizer {
    return &PerformanceOptimizer{
        cache:   make(map[string]interface{}),
        metrics: NewMetrics(),
    }
}

// StartProfiling starts CPU and memory profiling
func (po *PerformanceOptimizer) StartProfiling(ctx context.Context) {
    // CPU profiling
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                po.profileCPU()
                po.profileMemory()
            }
        }
    }()
}

// profileCPU performs CPU profiling
func (po *PerformanceOptimizer) profileCPU() {
    f, err := os.Create(fmt.Sprintf("cpu_profile_%d.prof", time.Now().Unix()))
    if err != nil {
        log.Printf("Failed to create CPU profile: %v", err)
        return
    }
    defer f.Close()
    
    if err := pprof.StartCPUProfile(f); err != nil {
        log.Printf("Failed to start CPU profile: %v", err)
        return
    }
    
    time.Sleep(10 * time.Second)
    pprof.StopCPUProfile()
}

// profileMemory performs memory profiling
func (po *PerformanceOptimizer) profileMemory() {
    f, err := os.Create(fmt.Sprintf("memory_profile_%d.prof", time.Now().Unix()))
    if err != nil {
        log.Printf("Failed to create memory profile: %v", err)
        return
    }
    defer f.Close()
    
    runtime.GC()
    if err := pprof.WriteHeapProfile(f); err != nil {
        log.Printf("Failed to write memory profile: %v", err)
    }
}

// CacheGet retrieves a value from cache
func (po *PerformanceOptimizer) CacheGet(key string) (interface{}, bool) {
    po.cacheMux.RLock()
    defer po.cacheMux.RUnlock()
    
    value, exists := po.cache[key]
    return value, exists
}

// CacheSet stores a value in cache
func (po *PerformanceOptimizer) CacheSet(key string, value interface{}, ttl time.Duration) {
    po.cacheMux.Lock()
    defer po.cacheMux.Unlock()
    
    po.cache[key] = value
    
    // Set expiration
    go func() {
        time.Sleep(ttl)
        po.cacheMux.Lock()
        delete(po.cache, key)
        po.cacheMux.Unlock()
    }()
}

// OptimizeGarbageCollection optimizes garbage collection
func (po *PerformanceOptimizer) OptimizeGarbageCollection() {
    // Set GOGC to optimize garbage collection
    runtime.GOMAXPROCS(runtime.NumCPU())
    
    // Monitor and adjust GC
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            // Adjust GOGC based on memory usage
            if m.Alloc > 100*1024*1024 { // 100MB
                runtime.GC()
            }
        }
    }()
}

// ConnectionPool manages database connection pooling
type ConnectionPool struct {
    maxConnections int
    connections    chan *sql.DB
    mu             sync.Mutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(maxConnections int) *ConnectionPool {
    return &ConnectionPool{
        maxConnections: maxConnections,
        connections:    make(chan *sql.DB, maxConnections),
    }
}

// GetConnection gets a connection from the pool
func (cp *ConnectionPool) GetConnection() (*sql.DB, error) {
    select {
    case conn := <-cp.connections:
        return conn, nil
    default:
        // Create new connection if pool is empty
        return cp.createConnection()
    }
}

// ReturnConnection returns a connection to the pool
func (cp *ConnectionPool) ReturnConnection(conn *sql.DB) {
    select {
    case cp.connections <- conn:
        // Connection returned to pool
    default:
        // Pool is full, close connection
        conn.Close()
    }
}

// createConnection creates a new database connection
func (cp *ConnectionPool) createConnection() (*sql.DB, error) {
    // Implementation would depend on your database driver
    return nil, nil
}
```

### **3. Monitoring and Alerting**

#### **Application Monitoring**
```go
// internal/monitoring/monitor.go
package monitoring

import (
    "context"
    "fmt"
    "log"
    "runtime"
    "time"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds application metrics
type Metrics struct {
    // HTTP metrics
    httpRequestsTotal   prometheus.Counter
    httpRequestDuration prometheus.Histogram
    httpRequestsInFlight prometheus.Gauge
    
    // Business metrics
    transactionsTotal    prometheus.Counter
    blocksCreated       prometheus.Counter
    activeWallets       prometheus.Gauge
    
    // System metrics
    memoryUsage         prometheus.Gauge
    cpuUsage            prometheus.Gauge
    goroutines          prometheus.Gauge
    databaseConnections prometheus.Gauge
}

// NewMetrics creates new application metrics
func NewMetrics() *Metrics {
    return &Metrics{
        httpRequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        }),
        httpRequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "Duration of HTTP requests",
            Buckets: prometheus.DefBuckets,
        }),
        httpRequestsInFlight: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "http_requests_in_flight",
            Help: "Number of HTTP requests currently being processed",
        }),
        transactionsTotal: promauto.NewCounter(prometheus.CounterOpts{
            Name: "transactions_total",
            Help: "Total number of transactions",
        }),
        blocksCreated: promauto.NewCounter(prometheus.CounterOpts{
            Name: "blocks_created_total",
            Help: "Total number of blocks created",
        }),
        activeWallets: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "active_wallets",
            Help: "Number of active wallets",
        }),
        memoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "memory_usage_bytes",
            Help: "Current memory usage in bytes",
        }),
        cpuUsage: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "cpu_usage_percent",
            Help: "Current CPU usage percentage",
        }),
        goroutines: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "goroutines",
            Help: "Number of active goroutines",
        }),
        databaseConnections: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "database_connections",
            Help: "Number of active database connections",
        }),
    }
}

// Monitor starts system monitoring
func (m *Metrics) Monitor(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.collectSystemMetrics()
        }
    }
}

// collectSystemMetrics collects system metrics
func (m *Metrics) collectSystemMetrics() {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    
    // Memory metrics
    m.memoryUsage.Set(float64(memStats.Alloc))
    
    // Goroutine metrics
    m.goroutines.Set(float64(runtime.NumGoroutine()))
    
    // CPU metrics (simplified - in production you'd use more sophisticated CPU monitoring)
    // m.cpuUsage.Set(getCPUUsage())
}

// HTTPMiddleware creates HTTP monitoring middleware
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Increment in-flight requests
        m.httpRequestsInFlight.Inc()
        defer m.httpRequestsInFlight.Dec()
        
        // Increment total requests
        m.httpRequestsTotal.Inc()
        
        // Call next handler
        next.ServeHTTP(w, r)
        
        // Record duration
        duration := time.Since(start).Seconds()
        m.httpRequestDuration.Observe(duration)
    })
}

// AlertManager manages alerts
type AlertManager struct {
    alerts chan Alert
    rules  []AlertRule
}

// Alert represents an alert
type Alert struct {
    Level     string
    Message   string
    Timestamp time.Time
    Labels    map[string]string
}

// AlertRule defines an alert rule
type AlertRule struct {
    Name      string
    Condition func() bool
    Level     string
    Message   string
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
    am := &AlertManager{
        alerts: make(chan Alert, 100),
        rules:  []AlertRule{},
    }
    
    // Add default alert rules
    am.addDefaultRules()
    
    return am
}

// addDefaultRules adds default alert rules
func (am *AlertManager) addDefaultRules() {
    am.rules = append(am.rules, AlertRule{
        Name: "high_memory_usage",
        Condition: func() bool {
            var memStats runtime.MemStats
            runtime.ReadMemStats(&memStats)
            return memStats.Alloc > 500*1024*1024 // 500MB
        },
        Level:   "warning",
        Message: "High memory usage detected",
    })
    
    am.rules = append(am.rules, AlertRule{
        Name: "high_goroutine_count",
        Condition: func() bool {
            return runtime.NumGoroutine() > 1000
        },
        Level:   "warning",
        Message: "High number of goroutines detected",
    })
}

// CheckAlerts checks all alert rules
func (am *AlertManager) CheckAlerts() {
    for _, rule := range am.rules {
        if rule.Condition() {
            alert := Alert{
                Level:     rule.Level,
                Message:   rule.Message,
                Timestamp: time.Now(),
                Labels:    map[string]string{"rule": rule.Name},
            }
            
            select {
            case am.alerts <- alert:
                log.Printf("Alert triggered: %s - %s", rule.Level, rule.Message)
            default:
                log.Printf("Alert channel full, dropping alert: %s", rule.Message)
            }
        }
    }
}

// GetAlerts returns alerts from the channel
func (am *AlertManager) GetAlerts() <-chan Alert {
    return am.alerts
}
```

### **4. High Availability**

#### **Load Balancing and Failover**
```go
// internal/ha/loadbalancer.go
package ha

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    "time"
)

// LoadBalancer implements load balancing
type LoadBalancer struct {
    backends []Backend
    strategy LoadBalancingStrategy
    mu       sync.RWMutex
}

// Backend represents a backend server
type Backend struct {
    URL           string
    Health        bool
    LastCheck     time.Time
    ResponseTime  time.Duration
    ActiveConnections int
}

// LoadBalancingStrategy defines load balancing strategies
type LoadBalancingStrategy interface {
    SelectBackend(backends []Backend) *Backend
}

// RoundRobinStrategy implements round-robin load balancing
type RoundRobinStrategy struct {
    current int
    mu      sync.Mutex
}

func (rr *RoundRobinStrategy) SelectBackend(backends []Backend) *Backend {
    rr.mu.Lock()
    defer rr.mu.Unlock()
    
    if len(backends) == 0 {
        return nil
    }
    
    // Find next healthy backend
    for i := 0; i < len(backends); i++ {
        index := (rr.current + i) % len(backends)
        if backends[index].Health {
            rr.current = (index + 1) % len(backends)
            return &backends[index]
        }
    }
    
    return nil
}

// LeastConnectionsStrategy implements least connections load balancing
type LeastConnectionsStrategy struct{}

func (lc *LeastConnectionsStrategy) SelectBackend(backends []Backend) *Backend {
    var selected *Backend
    minConnections := int(^uint(0) >> 1) // Max int
    
    for i := range backends {
        if backends[i].Health && backends[i].ActiveConnections < minConnections {
            selected = &backends[i]
            minConnections = backends[i].ActiveConnections
        }
    }
    
    return selected
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
    lb := &LoadBalancer{
        strategy: strategy,
        backends: []Backend{},
    }
    
    return lb
}

// AddBackend adds a backend to the load balancer
func (lb *LoadBalancer) AddBackend(url string) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    backend := Backend{
        URL:       url,
        Health:    true,
        LastCheck: time.Now(),
    }
    
    lb.backends = append(lb.backends, backend)
}

// RemoveBackend removes a backend from the load balancer
func (lb *LoadBalancer) RemoveBackend(url string) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    for i, backend := range lb.backends {
        if backend.URL == url {
            lb.backends = append(lb.backends[:i], lb.backends[i+1:]...)
            break
        }
    }
}

// GetBackend gets the next backend using the load balancing strategy
func (lb *LoadBalancer) GetBackend() *Backend {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    return lb.strategy.SelectBackend(lb.backends)
}

// HealthCheck performs health checks on all backends
func (lb *LoadBalancer) HealthCheck(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            lb.checkAllBackends()
        }
    }
}

// checkAllBackends checks health of all backends
func (lb *LoadBalancer) checkAllBackends() {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    for i := range lb.backends {
        go lb.checkBackendHealth(&lb.backends[i])
    }
}

// checkBackendHealth checks health of a single backend
func (lb *LoadBalancer) checkBackendHealth(backend *Backend) {
    start := time.Now()
    
    client := &http.Client{
        Timeout: 5 * time.Second,
    }
    
    resp, err := client.Get(backend.URL + "/health")
    if err != nil {
        backend.Health = false
        return
    }
    defer resp.Body.Close()
    
    backend.ResponseTime = time.Since(start)
    backend.LastCheck = time.Now()
    backend.Health = resp.StatusCode == http.StatusOK
}
```

### **5. Disaster Recovery**

#### **Backup and Recovery**
```go
// internal/recovery/backup.go
package recovery

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"
    "compress/gzip"
    "archive/tar"
    "io"
)

// BackupManager manages backup and recovery operations
type BackupManager struct {
    backupDir string
    retention time.Duration
}

// NewBackupManager creates a new backup manager
func NewBackupManager(backupDir string, retention time.Duration) *BackupManager {
    return &BackupManager{
        backupDir: backupDir,
        retention: retention,
    }
}

// CreateBackup creates a full backup
func (bm *BackupManager) CreateBackup(ctx context.Context) error {
    timestamp := time.Now().Format("2006-01-02_15-04-05")
    backupPath := filepath.Join(bm.backupDir, fmt.Sprintf("backup_%s.tar.gz", timestamp))
    
    // Create backup directory if it doesn't exist
    if err := os.MkdirAll(bm.backupDir, 0755); err != nil {
        return fmt.Errorf("failed to create backup directory: %v", err)
    }
    
    // Create backup file
    file, err := os.Create(backupPath)
    if err != nil {
        return fmt.Errorf("failed to create backup file: %v", err)
    }
    defer file.Close()
    
    // Create gzip writer
    gzipWriter := gzip.NewWriter(file)
    defer gzipWriter.Close()
    
    // Create tar writer
    tarWriter := tar.NewWriter(gzipWriter)
    defer tarWriter.Close()
    
    // Backup database
    if err := bm.backupDatabase(tarWriter); err != nil {
        return fmt.Errorf("failed to backup database: %v", err)
    }
    
    // Backup configuration files
    if err := bm.backupConfig(tarWriter); err != nil {
        return fmt.Errorf("failed to backup config: %v", err)
    }
    
    // Backup blockchain data
    if err := bm.backupBlockchainData(tarWriter); err != nil {
        return fmt.Errorf("failed to backup blockchain data: %v", err)
    }
    
    log.Printf("Backup created successfully: %s", backupPath)
    return nil
}

// backupDatabase backs up the database
func (bm *BackupManager) backupDatabase(tarWriter *tar.Writer) error {
    // Implementation would depend on your database system
    // For PostgreSQL, you might use pg_dump
    // For SQLite, you might copy the database file
    
    log.Println("Backing up database...")
    return nil
}

// backupConfig backs up configuration files
func (bm *BackupManager) backupConfig(tarWriter *tar.Writer) error {
    configFiles := []string{
        "config/config.yaml",
        "config/secrets.yaml",
        ".env",
    }
    
    for _, configFile := range configFiles {
        if err := bm.addFileToTar(tarWriter, configFile, "config/"); err != nil {
            return err
        }
    }
    
    return nil
}

// backupBlockchainData backs up blockchain data
func (bm *BackupManager) backupBlockchainData(tarWriter *tar.Writer) error {
    blockchainDataDir := "data/blockchain"
    
    return filepath.Walk(blockchainDataDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        if !info.IsDir() {
            return bm.addFileToTar(tarWriter, path, "blockchain/")
        }
        
        return nil
    })
}

// addFileToTar adds a file to the tar archive
func (bm *BackupManager) addFileToTar(tarWriter *tar.Writer, filePath, tarPath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    stat, err := file.Stat()
    if err != nil {
        return err
    }
    
    header := &tar.Header{
        Name:    tarPath + filepath.Base(filePath),
        Mode:    int64(stat.Mode()),
        Size:    stat.Size(),
        ModTime: stat.ModTime(),
    }
    
    if err := tarWriter.WriteHeader(header); err != nil {
        return err
    }
    
    if _, err := io.Copy(tarWriter, file); err != nil {
        return err
    }
    
    return nil
}

// RestoreBackup restores from a backup
func (bm *BackupManager) RestoreBackup(backupPath string) error {
    log.Printf("Restoring from backup: %s", backupPath)
    
    // Implementation would depend on your specific backup format
    // This is a simplified example
    
    return nil
}

// CleanupOldBackups removes old backups
func (bm *BackupManager) CleanupOldBackups() error {
    files, err := os.ReadDir(bm.backupDir)
    if err != nil {
        return err
    }
    
    cutoff := time.Now().Add(-bm.retention)
    
    for _, file := range files {
        if file.IsDir() {
            continue
        }
        
        info, err := file.Info()
        if err != nil {
            continue
        }
        
        if info.ModTime().Before(cutoff) {
            filePath := filepath.Join(bm.backupDir, file.Name())
            if err := os.Remove(filePath); err != nil {
                log.Printf("Failed to remove old backup %s: %v", filePath, err)
            } else {
                log.Printf("Removed old backup: %s", filePath)
            }
        }
    }
    
    return nil
}

// ScheduleBackups schedules regular backups
func (bm *BackupManager) ScheduleBackups(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := bm.CreateBackup(ctx); err != nil {
                log.Printf("Failed to create scheduled backup: %v", err)
            }
            
            if err := bm.CleanupOldBackups(); err != nil {
                log.Printf("Failed to cleanup old backups: %v", err)
            }
        }
    }
}
```

---

## 🚀 Hands-on Exercises

### **Exercise 1: Security Hardening**

Implement comprehensive security measures:
- Input validation and sanitization
- Authentication and authorization
- Rate limiting and DDoS protection
- Security headers and HTTPS
- Database security and encryption

### **Exercise 2: Performance Optimization**

Optimize your application for production:
- Implement caching strategies
- Optimize database queries
- Add connection pooling
- Profile and optimize critical paths
- Implement load balancing

### **Exercise 3: Monitoring and Alerting**

Set up comprehensive monitoring:
- Application metrics collection
- System resource monitoring
- Custom business metrics
- Alert rules and notifications
- Dashboard creation

### **Exercise 4: High Availability**

Implement high availability features:
- Load balancing configuration
- Health checks and failover
- Database replication
- Backup and recovery procedures
- Disaster recovery planning

---

## 📊 Assessment Criteria

### **Security Hardening (25%)**
- Comprehensive security measures
- Input validation and sanitization
- Authentication and authorization
- Security monitoring and logging
- Compliance with security standards

### **Performance Optimization (25%)**
- Application performance tuning
- Database optimization
- Caching implementation
- Resource utilization optimization
- Load testing and benchmarking

### **Monitoring and Alerting (20%)**
- Metrics collection and visualization
- Alert rules and notifications
- System health monitoring
- Performance monitoring
- Log aggregation and analysis

### **High Availability (20%)**
- Load balancing implementation
- Failover mechanisms
- Health checks and recovery
- Backup and restore procedures
- Disaster recovery planning

### **Production Deployment (10%)**
- Production environment setup
- Deployment procedures
- Configuration management
- Documentation and runbooks
- Operational procedures

---

## 🔧 Development Setup

### **Project Structure**
```
blockchain/
├── cmd/
│   └── main.go
├── internal/
│   ├── blockchain/
│   ├── api/
│   ├── security/
│   ├── performance/
│   ├── monitoring/
│   ├── ha/
│   └── recovery/
├── config/
│   ├── production/
│   └── staging/
├── scripts/
│   ├── backup.sh
│   └── deploy.sh
├── monitoring/
│   ├── prometheus/
│   ├── grafana/
│   └── alerts/
├── docs/
│   ├── deployment.md
│   ├── security.md
│   └── disaster-recovery.md
├── Makefile
└── docker-compose.yml
```

### **Getting Started**
1. Set up security hardening measures
2. Implement performance optimization
3. Configure monitoring and alerting
4. Set up high availability features
5. Create backup and recovery procedures
6. Deploy to production environment

---

## 📚 Additional Resources

### **Recommended Reading**
- "Site Reliability Engineering" by Google
- "The Phoenix Project" by Gene Kim
- "Building Microservices" by Sam Newman
- "Security Engineering" by Ross Anderson

### **Tools and Technologies**
- **Security**: OWASP, security scanning tools, penetration testing
- **Monitoring**: Prometheus, Grafana, ELK Stack, application metrics
- **Performance**: Load testing tools, profiling, optimization
- **High Availability**: Load balancers, clustering, failover
- **Backup**: Automated backup systems, disaster recovery

### **Online Resources**
- **OWASP**: Security best practices
- **Prometheus**: Monitoring and alerting
- **Grafana**: Metrics visualization
- **Site Reliability Engineering**: SRE practices

---

## 🎯 Success Checklist

- [ ] Implement comprehensive security measures
- [ ] Optimize application performance
- [ ] Set up monitoring and alerting
- [ ] Configure high availability features
- [ ] Create backup and recovery procedures
- [ ] Deploy to production environment
- [ ] Document operational procedures
- [ ] Test disaster recovery procedures
- [ ] Implement compliance measures
- [ ] Create runbooks and documentation

---

**Ready to make your blockchain production-ready? Let's start implementing production-grade features! 🚀**

Next: [Section 19: Course Project & Next Steps](./section19/README.md)
