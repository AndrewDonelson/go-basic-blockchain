# Section 16 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 16 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Unit Testing Purpose**
**Answer: B) To test individual components in isolation**

**Explanation**: Unit testing focuses on testing individual functions, methods, or components in isolation to ensure they work correctly without dependencies on other parts of the system.

### **Question 2: Test Coverage**
**Answer: C) 90%**

**Explanation**: 90% test coverage is the industry standard for production code, providing a good balance between thorough testing and development efficiency.

### **Question 3: Integration Testing**
**Answer: B) Testing how components work together**

**Explanation**: Integration testing verifies that different components of the system work together correctly, testing the interfaces between components.

### **Question 4: Performance Testing**
**Answer: B) To ensure the application meets performance requirements**

**Explanation**: Performance testing validates that the application meets specified performance criteria like response times, throughput, and resource usage.

### **Question 5: Test-Driven Development**
**Answer: B) Write tests, then write code, then refactor**

**Explanation**: TDD follows the "Red-Green-Refactor" cycle: write failing tests first, then write code to make tests pass, then refactor for improvement.

### **Question 6: Mock Objects**
**Answer: B) When testing components that depend on external systems**

**Explanation**: Mock objects are used to isolate the component under test from external dependencies, making tests faster and more reliable.

### **Question 7: Continuous Integration**
**Answer: B) To catch integration issues early**

**Explanation**: Continuous integration runs tests automatically on every code change, helping identify integration problems early in the development cycle.

### **Question 8: Security Testing**
**Answer: B) To identify vulnerabilities and security weaknesses**

**Explanation**: Security testing specifically focuses on finding security vulnerabilities and ensuring the application is protected against various attack vectors.

---

## **True/False Questions**

### **Question 9**
**Answer: False**

**Explanation**: Unit tests should test individual components in isolation. Testing multiple components together is integration testing.

### **Question 10**
**Answer: True**

**Explanation**: Test coverage measures what percentage of the code is executed when tests run, helping identify untested code paths.

### **Question 11**
**Answer: False**

**Explanation**: Performance testing should be done throughout development, not just at the end, to catch performance issues early.

### **Question 12**
**Answer: True**

**Explanation**: Mock objects replace external dependencies, making tests faster and more reliable by eliminating external factors.

### **Question 13**
**Answer: True**

**Explanation**: Continuous integration automatically runs the test suite on every code commit to ensure code quality.

### **Question 14**
**Answer: False**

**Explanation**: Security testing is important for all applications, not just financial ones, as all applications can have security vulnerabilities.

---

## **Practical Questions**

### **Question 15: Unit Test Implementation**

```go
package blockchain

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestBlockCreation(t *testing.T) {
    // Arrange
    index := 1
    timestamp := int64(1234567890)
    data := "Test block data"
    previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
    
    // Act
    block := NewBlock(index, timestamp, data, previousHash)
    
    // Assert
    assert.Equal(t, index, block.Index)
    assert.Equal(t, timestamp, block.Timestamp)
    assert.Equal(t, data, block.Data)
    assert.Equal(t, previousHash, block.PreviousHash)
    assert.NotEmpty(t, block.Hash)
    assert.Equal(t, 0, block.Nonce)
}

func TestBlockHashCalculation(t *testing.T) {
    block := &Block{
        Index:        1,
        Timestamp:    1234567890,
        Data:        "Test data",
        PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
        Nonce:       0,
    }
    
    hash := block.CalculateHash()
    
    assert.Len(t, hash, 64) // SHA-256 hash length
    assert.NotEqual(t, "", hash)
    
    // Test consistency
    hash2 := block.CalculateHash()
    assert.Equal(t, hash, hash2)
}

func TestBlockValidation(t *testing.T) {
    t.Run("valid block", func(t *testing.T) {
        block := &Block{
            Index:        1,
            Timestamp:    1234567890,
            Data:        "Valid data",
            PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
            Nonce:       0,
        }
        block.Hash = block.CalculateHash()
        
        err := block.Validate()
        assert.NoError(t, err)
    })
    
    t.Run("invalid hash", func(t *testing.T) {
        block := &Block{
            Index:        1,
            Timestamp:    1234567890,
            Data:        "Valid data",
            PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
            Nonce:       0,
            Hash:        "invalid_hash",
        }
        
        err := block.Validate()
        assert.Error(t, err)
    })
}

func TestBlockMining(t *testing.T) {
    block := &Block{
        Index:        1,
        Timestamp:    1234567890,
        Data:        "Mining test data",
        PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
        Nonce:       0,
    }
    
    difficulty := 4
    block.Mine(difficulty)
    
    // Verify mining results
    assert.True(t, block.IsValid())
    assert.True(t, strings.HasPrefix(block.Hash, strings.Repeat("0", difficulty)))
}
```

### **Question 16: Integration Test Design**

```go
package api

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gorilla/mux"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestGetBlocksEndpoint(t *testing.T) {
    // Arrange
    router := mux.NewRouter()
    blockchain := TestBlockchain(t)
    handler := NewBlockchainHandler(blockchain)
    
    router.HandleFunc("/api/v1/blocks", handler.GetBlocks).Methods("GET")
    
    // Act
    req, err := http.NewRequest("GET", "/api/v1/blocks", nil)
    require.NoError(t, err)
    
    rr := httptest.NewRecorder()
    router.ServeHTTP(rr, req)
    
    // Assert
    assert.Equal(t, http.StatusOK, rr.Code)
    
    var response []Block
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    require.NoError(t, err)
    
    assert.Len(t, response, len(blockchain.Blocks))
}

func TestCreateTransactionEndpoint(t *testing.T) {
    // Arrange
    router := mux.NewRouter()
    blockchain := TestBlockchain(t)
    handler := NewBlockchainHandler(blockchain)
    
    router.HandleFunc("/api/v1/transactions", handler.CreateTransaction).Methods("POST")
    
    transaction := &Transaction{
        From:        "wallet1",
        To:          "wallet2",
        Amount:      100.0,
        Description: "Test transaction",
    }
    
    transactionJSON, err := json.Marshal(transaction)
    require.NoError(t, err)
    
    // Act
    req, err := http.NewRequest("POST", "/api/v1/transactions", bytes.NewBuffer(transactionJSON))
    require.NoError(t, err)
    req.Header.Set("Content-Type", "application/json")
    
    rr := httptest.NewRecorder()
    router.ServeHTTP(rr, req)
    
    // Assert
    assert.Equal(t, http.StatusCreated, rr.Code)
    
    var response Transaction
    err = json.Unmarshal(rr.Body.Bytes(), &response)
    require.NoError(t, err)
    
    assert.Equal(t, transaction.From, response.From)
    assert.Equal(t, transaction.To, response.To)
    assert.Equal(t, transaction.Amount, response.Amount)
    assert.NotEmpty(t, response.Hash)
}

func TestAuthenticationMiddleware(t *testing.T) {
    router := mux.NewRouter()
    handler := NewBlockchainHandler(TestBlockchain(t))
    
    // Add authentication middleware
    router.Use(AuthenticationMiddleware)
    router.HandleFunc("/api/v1/blocks", handler.GetBlocks).Methods("GET")
    
    // Test without token
    req, err := http.NewRequest("GET", "/api/v1/blocks", nil)
    require.NoError(t, err)
    
    rr := httptest.NewRecorder()
    router.ServeHTTP(rr, req)
    
    assert.Equal(t, http.StatusUnauthorized, rr.Code)
    
    // Test with valid token
    req, err = http.NewRequest("GET", "/api/v1/blocks", nil)
    require.NoError(t, err)
    req.Header.Set("Authorization", "Bearer valid_token")
    
    rr = httptest.NewRecorder()
    router.ServeHTTP(rr, req)
    
    assert.Equal(t, http.StatusOK, rr.Code)
}
```

### **Question 17: Performance Test Strategy**

```javascript
// performance_tests/load_test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '2m', target: 10 },  // Ramp up
    { duration: '5m', target: 10 },  // Steady load
    { duration: '2m', target: 50 },  // Increase load
    { duration: '5m', target: 50 },  // High load
    { duration: '2m', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% under 500ms
    errors: ['rate<0.1'],             // Error rate under 10%
  },
};

const BASE_URL = 'http://localhost:8080';

export default function() {
  // Test API endpoints
  const responses = {
    blocks: http.get(`${BASE_URL}/api/v1/blocks`),
    transactions: http.get(`${BASE_URL}/api/v1/transactions`),
    network: http.get(`${BASE_URL}/api/v1/network/status`),
  };

  // Check responses
  check(responses.blocks, {
    'blocks status is 200': (r) => r.status === 200,
    'blocks response time < 200ms': (r) => r.timings.duration < 200,
  }) || errorRate.add(1);

  check(responses.transactions, {
    'transactions status is 200': (r) => r.status === 200,
    'transactions response time < 300ms': (r) => r.timings.duration < 300,
  }) || errorRate.add(1);

  sleep(1);
}

// Benchmark testing
export function benchmarkTest() {
  const payload = JSON.stringify({
    from: 'wallet1',
    to: 'wallet2',
    amount: 100.0,
    description: 'Benchmark transaction'
  });

  const response = http.post(`${BASE_URL}/api/v1/transactions`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(response, {
    'transaction created successfully': (r) => r.status === 201,
    'response time < 500ms': (r) => r.timings.duration < 500,
  }) || errorRate.add(1);
}
```

```go
// benchmark_test.go
package blockchain

import (
    "testing"
    "time"
)

func BenchmarkBlockCreation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        block := NewBlock(i, time.Now().Unix(), "Benchmark data", "previous_hash")
        block.CalculateHash()
    }
}

func BenchmarkBlockchainValidation(b *testing.B) {
    bc := NewBlockchain()
    
    // Add test blocks
    for i := 0; i < 100; i++ {
        bc.AddBlock("Test block data")
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bc.IsValid()
    }
}

func BenchmarkTransactionCreation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        tx := NewTransaction("wallet1", "wallet2", 100.0, "Benchmark transaction")
        tx.CalculateHash()
    }
}
```

### **Question 18: Security Test Implementation**

```go
package security

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSQLInjectionProtection(t *testing.T) {
    maliciousInput := "'; DROP TABLE blocks; --"
    
    sanitized := SanitizeInput(maliciousInput)
    assert.NotEqual(t, maliciousInput, sanitized)
    assert.NotContains(t, sanitized, "DROP TABLE")
    assert.NotContains(t, sanitized, "--")
}

func TestXSSProtection(t *testing.T) {
    maliciousScript := "<script>alert('xss')</script>"
    
    sanitized := SanitizeInput(maliciousScript)
    assert.NotContains(t, sanitized, "<script>")
    assert.NotContains(t, sanitized, "</script>")
    assert.Contains(t, sanitized, "&lt;script&gt;")
}

func TestInputValidation(t *testing.T) {
    t.Run("valid transaction", func(t *testing.T) {
        tx := &Transaction{
            From:        "wallet1",
            To:          "wallet2",
            Amount:      100.0,
            Description: "Valid transaction",
        }
        
        err := ValidateTransaction(tx)
        assert.NoError(t, err)
    })
    
    t.Run("negative amount", func(t *testing.T) {
        tx := &Transaction{
            From:        "wallet1",
            To:          "wallet2",
            Amount:      -100.0,
            Description: "Invalid transaction",
        }
        
        err := ValidateTransaction(tx)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "amount must be positive")
    })
    
    t.Run("empty addresses", func(t *testing.T) {
        tx := &Transaction{
            From:        "",
            To:          "",
            Amount:      100.0,
            Description: "Invalid transaction",
        }
        
        err := ValidateTransaction(tx)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "addresses cannot be empty")
    })
}

func TestAuthenticationBypass(t *testing.T) {
    // Test various authentication bypass attempts
    testCases := []struct {
        name     string
        token    string
        expected int
    }{
        {"no token", "", 401},
        {"invalid token", "invalid_token", 401},
        {"expired token", "expired_token", 401},
        {"valid token", "valid_token", 200},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            req, err := http.NewRequest("GET", "/api/v1/blocks", nil)
            require.NoError(t, err)
            
            if tc.token != "" {
                req.Header.Set("Authorization", "Bearer "+tc.token)
            }
            
            rr := httptest.NewRecorder()
            router.ServeHTTP(rr, req)
            
            assert.Equal(t, tc.expected, rr.Code)
        })
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

- **Excellent (90%+)**: 47+ points - You have mastered testing and quality assurance
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 17
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 16! 🎉**

Ready for the next challenge? Move on to [Section 17: Build System & Deployment](./section17/README.md)!
