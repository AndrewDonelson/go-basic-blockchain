# Section 8 Quiz Answers

## 📋 Answer Key

### **Multiple Choice Questions**
1. **B) Stateless communication** - RESTful APIs should be stateless
2. **B) POST** - POST is used to create new resources
3. **B) JWT tokens** - Most common for modern APIs
4. **B) To prevent abuse and ensure fair usage**
5. **B) Use URL path versioning (e.g., /api/v1/)**
6. **B) 201 Created** - Indicates successful resource creation
7. **A) Cross-Origin Resource Sharing**
8. **B) Swagger/OpenAPI** - Industry standard

### **True/False Questions**
9. **True** - RESTful APIs should be stateless
10. **False** - JWT tokens should be stored securely, not in localStorage
11. **True** - URLs should use nouns (e.g., /users, /blocks)
12. **False** - Rate limiting is important for all APIs
13. **True** - Consistent status codes are essential
14. **False** - Documentation is important for all APIs

### **Practical Questions**

#### **Question 15: Basic API Setup**
```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "github.com/gorilla/mux"
)

type APIServer struct {
    Router     *mux.Router
    Blockchain *Blockchain
}

func NewAPIServer(blockchain *Blockchain) *APIServer {
    return &APIServer{
        Router:     mux.NewRouter(),
        Blockchain: blockchain,
    }
}

func (api *APIServer) SetupRoutes() {
    api.Router.HandleFunc("/api/v1/blocks", api.GetBlocks).Methods("GET")
    api.Router.HandleFunc("/api/v1/transactions", api.CreateTransaction).Methods("POST")
    api.Router.HandleFunc("/api/v1/wallets", api.GetWallets).Methods("GET")
}

func (api *APIServer) GetBlocks(w http.ResponseWriter, r *http.Request) {
    blocks := api.Blockchain.GetBlocks()
    json.NewEncoder(w).Encode(blocks)
}

func (api *APIServer) CreateTransaction(w http.ResponseWriter, r *http.Request) {
    var tx Transaction
    if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    if err := api.Blockchain.AddTransaction(tx); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(tx)
}

func (api *APIServer) GetWallets(w http.ResponseWriter, r *http.Request) {
    wallets := api.Blockchain.GetWallets()
    json.NewEncoder(w).Encode(wallets)
}

func main() {
    blockchain := NewBlockchain("data")
    api := NewAPIServer(blockchain)
    api.SetupRoutes()
    
    log.Fatal(http.ListenAndServe(":8080", api.Router))
}
```

#### **Question 16: Authentication Middleware**
```go
type AuthMiddleware struct {
    JWTSecret string
}

func (auth *AuthMiddleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "No token provided", http.StatusUnauthorized)
            return
        }
        
        // Validate JWT token
        if !auth.validateToken(token) {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        next.ServeHTTP(w, r)
    }
}

func (auth *AuthMiddleware) validateToken(token string) bool {
    // JWT validation logic
    return true // Simplified
}
```

#### **Question 17: API Endpoints**
```go
// Comprehensive blockchain endpoints
func (api *APIServer) SetupBlockchainRoutes() {
    // Block endpoints
    api.Router.HandleFunc("/api/v1/blocks", api.GetBlocks).Methods("GET")
    api.Router.HandleFunc("/api/v1/blocks/{id}", api.GetBlock).Methods("GET")
    api.Router.HandleFunc("/api/v1/blocks", api.CreateBlock).Methods("POST")
    
    // Transaction endpoints
    api.Router.HandleFunc("/api/v1/transactions", api.GetTransactions).Methods("GET")
    api.Router.HandleFunc("/api/v1/transactions", api.CreateTransaction).Methods("POST")
    api.Router.HandleFunc("/api/v1/transactions/{id}", api.GetTransaction).Methods("GET")
    
    // Wallet endpoints
    api.Router.HandleFunc("/api/v1/wallets", api.GetWallets).Methods("GET")
    api.Router.HandleFunc("/api/v1/wallets", api.CreateWallet).Methods("POST")
    api.Router.HandleFunc("/api/v1/wallets/{address}", api.GetWallet).Methods("GET")
    
    // Network endpoints
    api.Router.HandleFunc("/api/v1/network/status", api.GetNetworkStatus).Methods("GET")
    api.Router.HandleFunc("/api/v1/network/peers", api.GetPeers).Methods("GET")
}
```

#### **Question 18: Security Implementation**
```go
type SecurityMiddleware struct {
    RateLimiter *RateLimiter
}

func (sec *SecurityMiddleware) RateLimit(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        clientIP := r.RemoteAddr
        if !sec.RateLimiter.Allow(clientIP) {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    }
}

func (sec *SecurityMiddleware) CORS(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    }
}
```

### **Bonus Challenge: Complete API System**
```go
// Complete RESTful API system with all features
type CompleteAPISystem struct {
    Server      *APIServer
    Auth        *AuthMiddleware
    Security    *SecurityMiddleware
    Logger      *Logger
    Monitor     *APIMonitor
    Docs        *APIDocumentation
}

func NewCompleteAPISystem(blockchain *Blockchain) *CompleteAPISystem {
    return &CompleteAPISystem{
        Server:   NewAPIServer(blockchain),
        Auth:     &AuthMiddleware{JWTSecret: "secret"},
        Security: &SecurityMiddleware{RateLimiter: NewRateLimiter()},
        Logger:   NewLogger(),
        Monitor:  NewAPIMonitor(),
        Docs:     NewAPIDocumentation(),
    }
}

func (cas *CompleteAPISystem) Start() {
    // Setup all middleware and routes
    cas.setupMiddleware()
    cas.setupRoutes()
    cas.setupDocumentation()
    
    // Start monitoring
    go cas.Monitor.Start()
    
    log.Fatal(http.ListenAndServe(":8080", cas.Server.Router))
}
```

---

**Great job completing Section 8! 🎉**

Ready for the next challenge? Move on to [Section 9: Enhanced Security Features](../section9/README.md)!
