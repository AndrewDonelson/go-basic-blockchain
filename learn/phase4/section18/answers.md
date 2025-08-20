# Section 18 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 18 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Security Headers**
**Answer: B) X-Frame-Options**

**Explanation**: The X-Frame-Options header prevents clickjacking attacks by controlling whether a browser should be allowed to render a page in a frame, iframe, embed, or object.

### **Question 2: Rate Limiting**
**Answer: B) To prevent abuse and DDoS attacks**

**Explanation**: Rate limiting protects applications from abuse, DDoS attacks, and ensures fair resource usage by limiting the number of requests a client can make in a given time period.

### **Question 3: Performance Optimization**
**Answer: B) Implementing connection pooling**

**Explanation**: Connection pooling is one of the most effective techniques for improving database performance as it reduces the overhead of creating and destroying database connections.

### **Question 4: Monitoring Metrics**
**Answer: D) All of the above**

**Explanation**: All types of metrics are important for comprehensive monitoring. Business metrics show value, system metrics show infrastructure health, and application metrics show application performance.

### **Question 5: High Availability**
**Answer: B) To ensure continuous service availability**

**Explanation**: The main goal of high availability is to ensure that services remain available even when individual components fail, providing continuous service to users.

### **Question 6: Disaster Recovery**
**Answer: A) The maximum acceptable time to restore service**

**Explanation**: Recovery Time Objective (RTO) is the maximum acceptable time to restore service after a disaster, defining how quickly the system must be back online.

### **Question 7: Load Balancing**
**Answer: B) Round robin**

**Explanation**: Round robin load balancing distributes requests evenly across all healthy servers in a sequential manner, providing fair distribution.

### **Question 8: Security Hardening**
**Answer: B) Users should have only the minimum access necessary**

**Explanation**: The principle of least privilege states that users should have only the minimum access necessary to perform their functions, reducing security risks.

---

## **True/False Questions**

### **Question 9**
**Answer: False**

**Explanation**: HTTPS should be used for all production applications, not just those handling sensitive data, as it provides encryption and authentication for all communications.

### **Question 10**
**Answer: True**

**Explanation**: Caching stores frequently accessed data in memory, reducing the need to query the database repeatedly, which significantly improves performance.

### **Question 11**
**Answer: False**

**Explanation**: Health checks are useful for both load balancers and monitoring systems to detect when services are unhealthy and trigger appropriate responses.

### **Question 12**
**Answer: False**

**Explanation**: Backup and recovery procedures should be tested in non-production environments first, then validated in production during maintenance windows.

### **Question 13**
**Answer: False**

**Explanation**: Input validation is necessary for all data inputs, including API endpoints, database queries, and any external data sources, not just user-facing forms.

### **Question 14**
**Answer: True**

**Explanation**: Monitoring and alerting should be set up before production deployment to ensure you can detect and respond to issues immediately when they occur.

---

## **Practical Questions**

### **Question 15: Security Implementation**

```go
// Security middleware implementation
package security

import (
    "net/http"
    "time"
    "strings"
    "golang.org/x/crypto/bcrypt"
    "github.com/golang-jwt/jwt/v5"
)

// SecureHeaders adds security headers
func SecureHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        
        next.ServeHTTP(w, r)
    })
}

// RateLimiter implements rate limiting
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
    sanitized := strings.ReplaceAll(input, "<script>", "")
    sanitized = strings.ReplaceAll(sanitized, "</script>", "")
    sanitized = strings.ReplaceAll(sanitized, "javascript:", "")
    sanitized = strings.ReplaceAll(sanitized, "onload=", "")
    sanitized = strings.ReplaceAll(sanitized, "onerror=", "")
    
    return sanitized
}

// JWT authentication
func GenerateToken(userID string, secret string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id": userID,
        "exp":     time.Now().Add(time.Hour * 24).Unix(),
        "iat":     time.Now().Unix(),
    })
    
    return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString, secret string) (string, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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

// Database security
func EnableSSL(db *sql.DB) error {
    _, err := db.Exec("SET sslmode=require")
    return err
}

func AuditLogging(db *sql.DB) error {
    createTable := `
    CREATE TABLE IF NOT EXISTS audit_log (
        id SERIAL PRIMARY KEY,
        user_id VARCHAR(255),
        action VARCHAR(255),
        ip_address INET,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )
    `
    _, err := db.Exec(createTable)
    return err
}
```

### **Question 16: Performance Optimization**

```go
// Performance optimization implementation
package performance

import (
    "sync"
    "time"
    "runtime"
    "database/sql"
)

// Cache implementation
type Cache struct {
    data map[string]interface{}
    mu   sync.RWMutex
    ttl  map[string]time.Time
}

func NewCache() *Cache {
    return &Cache{
        data: make(map[string]interface{}),
        ttl:  make(map[string]time.Time),
    }
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    if value, exists := c.data[key]; exists {
        if time.Now().Before(c.ttl[key]) {
            return value, true
        }
        // Expired, remove
        delete(c.data, key)
        delete(c.ttl, key)
    }
    
    return nil, false
}

func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.data[key] = value
    c.ttl[key] = time.Now().Add(duration)
}

// Connection pooling
type ConnectionPool struct {
    connections chan *sql.DB
    maxConn     int
}

func NewConnectionPool(maxConnections int) *ConnectionPool {
    return &ConnectionPool{
        connections: make(chan *sql.DB, maxConnections),
        maxConn:     maxConnections,
    }
}

func (cp *ConnectionPool) GetConnection() (*sql.DB, error) {
    select {
    case conn := <-cp.connections:
        return conn, nil
    default:
        return cp.createConnection()
    }
}

func (cp *ConnectionPool) ReturnConnection(conn *sql.DB) {
    select {
    case cp.connections <- conn:
        // Connection returned to pool
    default:
        // Pool is full, close connection
        conn.Close()
    }
}

// Profiling
func StartProfiling() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            // Log memory usage
            log.Printf("Memory: Alloc=%d MiB, Sys=%d MiB", 
                m.Alloc/1024/1024, m.Sys/1024/1024)
            
            // Log goroutine count
            log.Printf("Goroutines: %d", runtime.NumGoroutine())
        }
    }()
}

// Load testing helper
func LoadTest(url string, requests int, concurrency int) {
    semaphore := make(chan bool, concurrency)
    var wg sync.WaitGroup
    
    for i := 0; i < requests; i++ {
        wg.Add(1)
        semaphore <- true
        
        go func() {
            defer wg.Done()
            defer func() { <-semaphore }()
            
            start := time.Now()
            resp, err := http.Get(url)
            duration := time.Since(start)
            
            if err != nil {
                log.Printf("Request failed: %v", err)
            } else {
                log.Printf("Request completed in %v, status: %d", duration, resp.StatusCode)
                resp.Body.Close()
            }
        }()
    }
    
    wg.Wait()
}
```

### **Question 17: Monitoring Setup**

```go
// Monitoring implementation
package monitoring

import (
    "context"
    "time"
    "runtime"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics collection
type Metrics struct {
    httpRequestsTotal   prometheus.Counter
    httpRequestDuration prometheus.Histogram
    memoryUsage         prometheus.Gauge
    goroutines          prometheus.Gauge
    transactionsTotal   prometheus.Counter
    blocksCreated       prometheus.Counter
}

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
        memoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "memory_usage_bytes",
            Help: "Current memory usage in bytes",
        }),
        goroutines: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "goroutines",
            Help: "Number of active goroutines",
        }),
        transactionsTotal: promauto.NewCounter(prometheus.CounterOpts{
            Name: "transactions_total",
            Help: "Total number of transactions",
        }),
        blocksCreated: promauto.NewCounter(prometheus.CounterOpts{
            Name: "blocks_created_total",
            Help: "Total number of blocks created",
        }),
    }
}

// HTTP monitoring middleware
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        m.httpRequestsTotal.Inc()
        
        next.ServeHTTP(w, r)
        
        duration := time.Since(start).Seconds()
        m.httpRequestDuration.Observe(duration)
    })
}

// System monitoring
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

func (m *Metrics) collectSystemMetrics() {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    
    m.memoryUsage.Set(float64(memStats.Alloc))
    m.goroutines.Set(float64(runtime.NumGoroutine()))
}

// Alert manager
type AlertManager struct {
    alerts chan Alert
    rules  []AlertRule
}

type Alert struct {
    Level     string
    Message   string
    Timestamp time.Time
}

type AlertRule struct {
    Name      string
    Condition func() bool
    Level     string
    Message   string
}

func NewAlertManager() *AlertManager {
    am := &AlertManager{
        alerts: make(chan Alert, 100),
        rules:  []AlertRule{},
    }
    
    // Add default rules
    am.rules = append(am.rules, AlertRule{
        Name: "high_memory_usage",
        Condition: func() bool {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            return m.Alloc > 500*1024*1024 // 500MB
        },
        Level:   "warning",
        Message: "High memory usage detected",
    })
    
    return am
}

func (am *AlertManager) CheckAlerts() {
    for _, rule := range am.rules {
        if rule.Condition() {
            alert := Alert{
                Level:     rule.Level,
                Message:   rule.Message,
                Timestamp: time.Now(),
            }
            
            select {
            case am.alerts <- alert:
                log.Printf("Alert: %s - %s", rule.Level, rule.Message)
            default:
                log.Printf("Alert channel full, dropping alert")
            }
        }
    }
}
```

### **Question 18: High Availability Design**

```go
// High availability implementation
package ha

import (
    "context"
    "net/http"
    "sync"
    "time"
)

// Load balancer
type LoadBalancer struct {
    backends []Backend
    strategy LoadBalancingStrategy
    mu       sync.RWMutex
}

type Backend struct {
    URL           string
    Health        bool
    LastCheck     time.Time
    ResponseTime  time.Duration
}

type LoadBalancingStrategy interface {
    SelectBackend(backends []Backend) *Backend
}

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

func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
    return &LoadBalancer{
        strategy: strategy,
        backends: []Backend{},
    }
}

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

func (lb *LoadBalancer) GetBackend() *Backend {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    return lb.strategy.SelectBackend(lb.backends)
}

// Health checks
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

func (lb *LoadBalancer) checkAllBackends() {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    for i := range lb.backends {
        go lb.checkBackendHealth(&lb.backends[i])
    }
}

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

// Database replication
type DatabaseReplica struct {
    Primary   *sql.DB
    Replicas  []*sql.DB
    current   int
    mu        sync.Mutex
}

func NewDatabaseReplica(primary *sql.DB, replicas []*sql.DB) *DatabaseReplica {
    return &DatabaseReplica{
        Primary:  primary,
        Replicas: replicas,
        current:  0,
    }
}

func (dr *DatabaseReplica) GetReadConnection() *sql.DB {
    dr.mu.Lock()
    defer dr.mu.Unlock()
    
    if len(dr.Replicas) == 0 {
        return dr.Primary
    }
    
    conn := dr.Replicas[dr.current]
    dr.current = (dr.current + 1) % len(dr.Replicas)
    
    return conn
}

func (dr *DatabaseReplica) GetWriteConnection() *sql.DB {
    return dr.Primary
}

// Backup and recovery
type BackupManager struct {
    backupDir string
    retention time.Duration
}

func NewBackupManager(backupDir string, retention time.Duration) *BackupManager {
    return &BackupManager{
        backupDir: backupDir,
        retention: retention,
    }
}

func (bm *BackupManager) CreateBackup(ctx context.Context) error {
    timestamp := time.Now().Format("2006-01-02_15-04-05")
    backupPath := filepath.Join(bm.backupDir, fmt.Sprintf("backup_%s.sql", timestamp))
    
    // Implementation would depend on your database system
    log.Printf("Creating backup: %s", backupPath)
    
    return nil
}

func (bm *BackupManager) RestoreBackup(backupPath string) error {
    log.Printf("Restoring from backup: %s", backupPath)
    
    // Implementation would depend on your database system
    
    return nil
}

func (bm *BackupManager) ScheduleBackups(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := bm.CreateBackup(ctx); err != nil {
                log.Printf("Failed to create backup: %v", err)
            }
        }
    }
}
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on implementation completeness

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have mastered production readiness
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 19
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 18! 🎉**

Ready for the next challenge? Move on to [Section 19: Course Project & Next Steps](./section19/README.md)!
