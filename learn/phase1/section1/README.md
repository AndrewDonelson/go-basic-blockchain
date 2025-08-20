# Section 1: Course Introduction & Setup

## 🎯 Welcome to Your Blockchain Journey!

Welcome to the first section of your blockchain development journey! In this section, we'll set up your development environment and get you ready to build a complete blockchain from scratch using Go.

### **What You'll Learn in This Section**

- Course overview and learning objectives
- Development environment setup
- Project structure walkthrough
- Git basics and version control
- Your first "Hello Blockchain" program

### **Section Overview**

This section is designed to get you up and running quickly. We'll cover the essential tools and concepts you need before diving into blockchain development.

---

## 📋 Course Overview

### **Course Objectives**

By the end of this course, you will have built:

1. **A Complete Blockchain**: From basic blocks to advanced consensus
2. **Advanced Consensus Algorithm**: The sophisticated Helios algorithm
3. **Professional APIs**: RESTful APIs with authentication
4. **User Interfaces**: Web-based blockchain explorers
5. **P2P Networking**: Multi-node blockchain networks
6. **Production-Ready Code**: Deployable blockchain applications

### **Learning Approach**

- **Hands-on Learning**: Build everything from scratch
- **Progressive Complexity**: Start simple, build to advanced
- **Real-World Skills**: Learn production-ready practices
- **Portfolio Project**: Create something you can showcase

---

## 🛠️ Development Environment Setup

### **Required Tools**

#### **1. Go Programming Language**
- **Version**: Go 1.22 or later
- **Download**: https://golang.org/dl/
- **Installation**: Follow the official installation guide for your OS

#### **2. Code Editor**
- **VS Code** (Recommended): https://code.visualstudio.com/
- **GoLand**: https://www.jetbrains.com/go/
- **Vim/Emacs**: If you prefer command-line editors

#### **3. Git Version Control**
- **Download**: https://git-scm.com/
- **GitHub Account**: https://github.com/ (for code hosting)

#### **4. Terminal/Command Line**
- **Windows**: PowerShell or Git Bash
- **macOS**: Terminal
- **Linux**: Your preferred terminal

### **Installation Steps**

#### **Step 1: Install Go**

**Windows:**
```bash
# Download and run the installer from golang.org
# Add Go to your PATH environment variable
```

**macOS:**
```bash
# Using Homebrew
brew install go

# Or download from golang.org
```

**Linux:**
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install golang-go

# Or download from golang.org
```

#### **Step 2: Verify Go Installation**

```bash
go version
# Should output: go version go1.22.x linux/amd64 (or similar)
```

#### **Step 3: Set Up Go Workspace**

```bash
# Create your Go workspace
mkdir ~/go
mkdir ~/go/src
mkdir ~/go/bin
mkdir ~/go/pkg

# Set GOPATH environment variable
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

#### **Step 4: Install VS Code Extensions**

1. Install VS Code
2. Install the Go extension: `golang.go`
3. Install additional recommended extensions:
   - Go Test Explorer
   - Go Outliner
   - Go Doc

---

## 📁 Project Structure Walkthrough

### **Understanding the Project Structure**

Our blockchain project will follow a professional Go project structure:

```
go-basic-blockchain/
├── cmd/                    # Command-line applications
│   ├── chaind/            # Main blockchain daemon
│   └── gbb-cli/           # Command-line interface
├── internal/              # Private application code
│   ├── helios/            # Helios consensus algorithm
│   ├── menu/              # Interactive menu system
│   └── progress/          # Progress indicator
├── sdk/                   # Software development kit
│   ├── blockchain.go      # Main blockchain implementation
│   ├── block.go           # Block structure and methods
│   ├── wallet.go          # Wallet system
│   ├── api.go             # RESTful API
│   └── ...                # Other core components
├── docs/                  # Documentation
├── scripts/               # Build and utility scripts
├── data/                  # Blockchain data storage
├── go.mod                 # Go module definition
├── go.sum                 # Dependency checksums
├── Makefile               # Build automation
└── README.md              # Project documentation
```

### **Key Directories Explained**

#### **`cmd/` - Command Applications**
- Contains the main entry points for your applications
- `chaind/`: The main blockchain daemon that runs the node
- `gbb-cli/`: Command-line interface for blockchain operations

#### **`internal/` - Private Code**
- Code that's specific to this application
- Not meant to be imported by other projects
- Contains advanced features like the Helios consensus

#### **`sdk/` - Software Development Kit**
- The core blockchain implementation
- Public APIs and interfaces
- Reusable components

#### **`docs/` - Documentation**
- Comprehensive documentation
- API references
- Architecture guides

---

## 🔧 Git Basics and Version Control

### **Why Version Control Matters**

Version control is essential for:
- Tracking changes to your code
- Collaborating with others
- Reverting to previous versions
- Understanding code evolution

### **Basic Git Commands**

#### **Initializing a Repository**

```bash
# Create a new directory for your project
mkdir go-basic-blockchain
cd go-basic-blockchain

# Initialize a Git repository
git init

# Create your first commit
echo "# Go Basic Blockchain" > README.md
git add README.md
git commit -m "Initial commit: Project setup"
```

#### **Essential Git Workflow**

```bash
# Check status of your repository
git status

# Add files to staging area
git add .

# Commit changes with a descriptive message
git commit -m "Add blockchain core implementation"

# View commit history
git log --oneline

# Create and switch to a new branch
git checkout -b feature/new-feature

# Switch back to main branch
git checkout main

# Merge feature branch
git merge feature/new-feature
```

#### **Working with Remote Repositories**

```bash
# Add a remote repository (GitHub, GitLab, etc.)
git remote add origin https://github.com/yourusername/go-basic-blockchain.git

# Push your code to remote
git push -u origin main

# Pull latest changes
git pull origin main
```

### **Git Best Practices**

1. **Commit Frequently**: Make small, focused commits
2. **Write Clear Messages**: Use descriptive commit messages
3. **Use Branches**: Create feature branches for new work
4. **Review Changes**: Always review before committing
5. **Keep History Clean**: Use interactive rebase if needed

---

## 🚀 Your First "Hello Blockchain" Program

### **Creating Your First Go Program**

Let's create a simple program to verify your setup and introduce basic Go concepts.

#### **Step 1: Create the Project**

```bash
# Create project directory
mkdir hello-blockchain
cd hello-blockchain

# Initialize Go module
go mod init hello-blockchain
```

#### **Step 2: Create the Main Program**

Create a file called `main.go`:

```go
package main

import (
    "fmt"
    "time"
)

// Block represents a basic blockchain block
type Block struct {
    Index     int
    Timestamp time.Time
    Data      string
    Hash      string
}

// Blockchain represents a simple blockchain
type Blockchain struct {
    Blocks []*Block
}

// NewBlock creates a new block
func NewBlock(index int, data string) *Block {
    return &Block{
        Index:     index,
        Timestamp: time.Now(),
        Data:      data,
        Hash:      fmt.Sprintf("hash-%d", index), // Simplified hash
    }
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(data string) {
    index := len(bc.Blocks)
    block := NewBlock(index, data)
    bc.Blocks = append(bc.Blocks, block)
}

// Display displays all blocks in the blockchain
func (bc *Blockchain) Display() {
    fmt.Println("=== Blockchain ===")
    for _, block := range bc.Blocks {
        fmt.Printf("Block #%d\n", block.Index)
        fmt.Printf("Timestamp: %s\n", block.Timestamp.Format(time.RFC3339))
        fmt.Printf("Data: %s\n", block.Data)
        fmt.Printf("Hash: %s\n", block.Hash)
        fmt.Println("---")
    }
}

func main() {
    fmt.Println("🚀 Hello Blockchain!")
    fmt.Println("Creating your first blockchain...\n")

    // Create a new blockchain
    blockchain := &Blockchain{}

    // Add some blocks
    blockchain.AddBlock("Genesis Block - The beginning of our blockchain!")
    blockchain.AddBlock("Second Block - Learning Go and blockchain!")
    blockchain.AddBlock("Third Block - Building something amazing!")

    // Display the blockchain
    blockchain.Display()

    fmt.Println("✅ Your first blockchain is ready!")
    fmt.Println("In the next sections, we'll build a real blockchain with:")
    fmt.Println("- Cryptographic hashing")
    fmt.Println("- Proof of work mining")
    fmt.Println("- Transaction processing")
    fmt.Println("- Advanced consensus algorithms")
}
```

#### **Step 3: Run Your Program**

```bash
# Run the program
go run main.go
```

You should see output similar to:

```
🚀 Hello Blockchain!
Creating your first blockchain...

=== Blockchain ===
Block #0
Timestamp: 2024-01-15T10:30:00Z
Data: Genesis Block - The beginning of our blockchain!
Hash: hash-0
---
Block #1
Timestamp: 2024-01-15T10:30:00Z
Data: Second Block - Learning Go and blockchain!
Hash: hash-1
---
Block #2
Timestamp: 2024-01-15T10:30:00Z
Data: Third Block - Building something amazing!
Hash: hash-2
---

✅ Your first blockchain is ready!
In the next sections, we'll build a real blockchain with:
- Cryptographic hashing
- Proof of work mining
- Transaction processing
- Advanced consensus algorithms
```

---

## 📚 Key Concepts Introduced

### **Go Programming Concepts**

1. **Packages**: Code organization in Go
2. **Structs**: Custom data types
3. **Methods**: Functions attached to types
4. **Slices**: Dynamic arrays
5. **Pointers**: Memory references
6. **Time handling**: Working with dates and times

### **Blockchain Concepts**

1. **Blocks**: Basic units of blockchain data
2. **Chain**: Linking blocks together
3. **Hashing**: Creating unique identifiers
4. **Genesis Block**: The first block in a chain

---

## 🎯 Section Summary

In this section, you've learned:

✅ How to set up your Go development environment
✅ Basic Git version control concepts
✅ Go project structure and organization
✅ How to create and run your first Go program
✅ Basic blockchain concepts and terminology

### **Next Steps**

1. Complete the hands-on exercises below
2. Take the quiz to test your understanding
3. Move on to [Section 2: Go Fundamentals for Blockchain](../section2/README.md)

---

## 🛠️ Hands-On Exercises

### **Exercise 1: Environment Verification**

1. Verify your Go installation:
   ```bash
   go version
   go env
   ```

2. Create a simple Go program that prints "Hello, Go!" and run it.

### **Exercise 2: Git Practice**

1. Create a new Git repository
2. Add a README.md file
3. Make your first commit
4. Create a new branch and make changes
5. Merge your branch back to main

### **Exercise 3: Enhanced Hello Blockchain**

Modify the Hello Blockchain program to:
1. Add a "Previous Hash" field to each block
2. Implement a simple hash function using SHA-256
3. Link blocks by storing the previous block's hash
4. Add validation to ensure chain integrity

### **Exercise 4: Project Setup**

1. Create the project structure for your blockchain
2. Initialize a Go module
3. Set up Git repository
4. Create initial documentation

---

## 📝 Quiz

Ready to test your knowledge? Take the [Section 1 Quiz](./quiz.md) to verify your understanding of the concepts covered in this section.

---

**Congratulations! You've completed Section 1. You're now ready to dive deeper into Go programming and blockchain fundamentals in [Section 2](../section2/README.md)! 🚀**
