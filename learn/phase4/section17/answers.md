# Section 17 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 17 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Makefile Purpose**
**Answer: B) To automate build, test, and deployment tasks**

**Explanation**: Makefiles are used to automate repetitive tasks in software development, including building applications, running tests, and managing deployment processes.

### **Question 2: Cross-Compilation**
**Answer: B) Build binaries for different platforms from one machine**

**Explanation**: Cross-compilation allows developers to build applications for different operating systems and architectures from a single development machine.

### **Question 3: Docker Multi-Stage Builds**
**Answer: B) They create smaller production images by excluding build tools**

**Explanation**: Multi-stage builds separate the build environment from the runtime environment, resulting in smaller, more secure production images.

### **Question 4: CI/CD Pipeline**
**Answer: A) Continuous Integration / Continuous Deployment**

**Explanation**: CI/CD stands for Continuous Integration (automated testing) and Continuous Deployment (automated deployment) of software changes.

### **Question 5: Semantic Versioning**
**Answer: C) Bug fixes and minor improvements**

**Explanation**: In semantic versioning, PATCH versions (1.0.1, 1.0.2) indicate backward-compatible bug fixes and minor improvements.

### **Question 6: Kubernetes Deployment**
**Answer: B) To manage and scale application replicas**

**Explanation**: Kubernetes Deployments manage the lifecycle of application replicas, including scaling, updates, and rollbacks.

### **Question 7: Health Checks**
**Answer: B) To verify that the application is running correctly**

**Explanation**: Health checks verify that applications are functioning properly and can respond to requests, enabling automatic recovery.

### **Question 8: Release Management**
**Answer: B) To ensure reliable and repeatable software releases**

**Explanation**: Release management ensures that software releases are consistent, reliable, and can be repeated across different environments.

---

## **True/False Questions**

### **Question 9**
**Answer: False**

**Explanation**: Makefiles can be used for any programming language or build process, not just Go applications.

### **Question 10**
**Answer: True**

**Explanation**: Docker containers are typically much smaller than virtual machines because they share the host OS kernel and don't include a full operating system.

### **Question 11**
**Answer: True**

**Explanation**: Continuous Integration automatically runs the test suite whenever code is committed to the repository.

### **Question 12**
**Answer: True**

**Explanation**: Kubernetes can automatically scale applications up or down based on CPU usage, memory usage, or custom metrics.

### **Question 13**
**Answer: True**

**Explanation**: Semantic versioning provides clear information about the nature of changes, helping users understand what to expect from updates.

### **Question 14**
**Answer: False**

**Explanation**: While manual deployments can be used for critical systems, automated deployments with proper testing and rollback capabilities are generally safer and more reliable.

---

## **Practical Questions**

### **Question 15: Makefile Implementation**

```makefile
# Professional Makefile for Blockchain Application
.PHONY: help build build-all test lint clean docker-build docker-run deploy

# Variables
APP_NAME := blockchain
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD)
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Go build settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Docker settings
DOCKER_IMAGE := $(APP_NAME):$(VERSION)
DOCKER_LATEST := $(APP_NAME):latest

# Build directories
BUILD_DIR := build
BIN_DIR := $(BUILD_DIR)/bin

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build for current platform
	@echo "Building $(APP_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) ./cmd/main.go
	@echo "Build complete: $(BIN_DIR)/$(APP_NAME)"

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BIN_DIR)
	$(MAKE) build GOOS=linux GOARCH=amd64
	$(MAKE) build GOOS=linux GOARCH=arm64
	$(MAKE) build GOOS=darwin GOARCH=amd64
	$(MAKE) build GOOS=darwin GOARCH=arm64
	$(MAKE) build GOOS=windows GOARCH=amd64
	@echo "All builds complete"

# Test and lint targets
test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Test coverage report: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Cleanup targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image: $(DOCKER_IMAGE)"
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_LATEST) .
	@echo "Docker build complete"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 --name $(APP_NAME) $(DOCKER_IMAGE)

docker-stop: ## Stop Docker container
	@echo "Stopping Docker container..."
	docker stop $(APP_NAME) || true
	docker rm $(APP_NAME) || true

# Development workflow
dev: fmt lint test build ## Development workflow: format, lint, test, build

ci: test lint build docker-build ## CI workflow: test, lint, build, docker
```

### **Question 16: Docker Containerization**

```dockerfile
# Multi-stage Dockerfile for Blockchain Application
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=$(git describe --tags --always --dirty) \
              -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S') \
              -X main.GitCommit=$(git rev-parse --short HEAD)" \
    -a -installsuffix cgo -o main ./cmd/main.go

# Production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S blockchain && \
    adduser -u 1001 -S blockchain -G blockchain

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .

# Copy configuration files
COPY --from=builder /app/config ./config

# Change ownership to non-root user
RUN chown -R blockchain:blockchain /app

# Switch to non-root user
USER blockchain

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]
```

### **Question 17: CI/CD Pipeline Design**

```yaml
# .github/workflows/ci-cd.yml
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]
  release:
    types: [ published ]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: [1.21, 1.22]

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ matrix.go-version }}

    - name: Install dependencies
      run: go mod download

    - name: Run tests
      run: make test

    - name: Run linter
      run: make lint

    - name: Run security scan
      run: make security-scan

    - name: Upload coverage reports
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out

  build:
    name: Build
    runs-on: ubuntu-latest
    needs: test
    if: github.event_name == 'push' || github.event_name == 'release'

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Log in to Container Registry
      uses: docker/login-action@v3
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
        tags: |
          type=ref,event=branch
          type=ref,event=pr
          type=semver,pattern={{version}}
          type=semver,pattern={{major}}.{{minor}}
          type=sha

    - name: Build and push Docker image
      uses: docker/build-push-action@v5
      with:
        context: .
        push: true
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        cache-from: type=gha
        cache-to: type=gha,mode=max

  deploy-staging:
    name: Deploy to Staging
    runs-on: ubuntu-latest
    needs: build
    if: github.ref == 'refs/heads/main'
    environment: staging

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up kubectl
      uses: azure/setup-kubectl@v3
      with:
        version: 'latest'

    - name: Configure kubectl
      run: |
        echo "${{ secrets.KUBE_CONFIG_STAGING }}" | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig

    - name: Deploy to staging
      run: make deploy-staging

    - name: Verify deployment
      run: |
        kubectl rollout status deployment/blockchain -n blockchain-staging
        kubectl get pods -n blockchain-staging

  deploy-production:
    name: Deploy to Production
    runs-on: ubuntu-latest
    needs: [build, deploy-staging]
    if: github.event_name == 'release'
    environment: production

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up kubectl
      uses: azure/setup-kubectl@v3
      with:
        version: 'latest'

    - name: Configure kubectl
      run: |
        echo "${{ secrets.KUBE_CONFIG_PROD }}" | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig

    - name: Deploy to production
      run: make deploy-prod

    - name: Verify production deployment
      run: |
        kubectl rollout status deployment/blockchain -n blockchain-prod
        kubectl get pods -n blockchain-prod

  notify:
    name: Notify
    runs-on: ubuntu-latest
    needs: [deploy-production]
    if: always()

    steps:
    - name: Notify Slack
      uses: 8398a7/action-slack@v3
      with:
        status: ${{ job.status }}
        channel: '#deployments'
        webhook_url: ${{ secrets.SLACK_WEBHOOK }}
      if: always()
```

### **Question 18: Kubernetes Deployment**

```yaml
# k8s/prod/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: blockchain
  namespace: blockchain-prod
  labels:
    app: blockchain
    version: v1.0.0
spec:
  replicas: 3
  selector:
    matchLabels:
      app: blockchain
  template:
    metadata:
      labels:
        app: blockchain
        version: v1.0.0
    spec:
      containers:
      - name: blockchain
        image: ghcr.io/your-org/blockchain:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: CONFIG_FILE
          value: "/app/config/config.yaml"
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: blockchain-secrets
              key: database-url
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        - name: data
          mountPath: /app/data
      volumes:
      - name: config
        configMap:
          name: blockchain-config
      - name: data
        persistentVolumeClaim:
          claimName: blockchain-pvc
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
        fsGroup: 1001
```

```yaml
# k8s/prod/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: blockchain-service
  namespace: blockchain-prod
spec:
  selector:
    app: blockchain
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: ClusterIP
```

```yaml
# k8s/prod/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: blockchain-ingress
  namespace: blockchain-prod
  annotations:
    kubernetes.io/ingress.class: "nginx"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - blockchain.yourdomain.com
    secretName: blockchain-tls
  rules:
  - host: blockchain.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: blockchain-service
            port:
              number: 80
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on implementation completeness

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have mastered build systems and deployment
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 18
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 17! 🎉**

Ready for the next challenge? Move on to [Section 18: Production Readiness](./section18/README.md)!
