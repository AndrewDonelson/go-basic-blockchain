# Section 1 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 1 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Go Installation**
**Answer: B) `go version`**

**Explanation**: The `go version` command displays the installed Go version and platform information. This is the standard way to verify that Go is properly installed.

### **Question 2: Go Workspace Structure**
**Answer: D) `lib/`**

**Explanation**: The standard Go workspace directories are `src/`, `bin/`, and `pkg/`. The `lib/` directory is not part of the standard Go workspace structure.

### **Question 3: Go Modules**
**Answer: A) `go.mod`**

**Explanation**: The `go.mod` file defines a Go module and lists its dependencies. It's the primary file for module management in Go.

### **Question 4: Git Basics**
**Answer: B) `git add`**

**Explanation**: The `git add` command stages files for commit. It prepares files to be included in the next commit.

### **Question 5: Blockchain Structure**
**Answer: B) Genesis Block**

**Explanation**: The genesis block is the first block in a blockchain. It's special because it doesn't have a previous block to reference.

### **Question 6: Go Project Structure**
**Answer: B) `cmd/`**

**Explanation**: The `cmd/` directory typically contains the main entry points for Go applications. Each subdirectory represents a different executable.

### **Question 7: Version Control**
**Answer: B) To specify which files Git should ignore**

**Explanation**: The `.gitignore` file tells Git which files and directories to ignore when tracking changes.

### **Question 8: Go Structs**
**Answer: A) `type`**

**Explanation**: In Go, the `type` keyword is used to define custom data types, including structs. The syntax is `type StructName struct`.

---

## **True/False Questions**

### **Question 9**
**Answer: True**

**Explanation**: Go is a compiled programming language that produces machine code. The `go build` command compiles Go source code into executable binaries.

### **Question 10**
**Answer: True**

**Explanation**: Since Git 2.28, the `main` branch is the default branch for new repositories, replacing the previous default of `master`.

### **Question 11**
**Answer: True**

**Explanation**: A blockchain is essentially a distributed database that stores data in a linked list structure, where each block contains a reference to the previous block.

### **Question 12**
**Answer: False**

**Explanation**: The `internal/` directory in Go projects contains private code that cannot be imported by other projects. This is enforced by the Go compiler.

### **Question 13**
**Answer: False**

**Explanation**: While VS Code is popular for Go development, other editors like GoLand, Vim, and Emacs are also excellent choices. The choice depends on personal preference.

### **Question 14**
**Answer: True**

**Explanation**: Making frequent, small commits is a best practice in Git. It makes it easier to track changes, revert specific changes, and collaborate with others.

---

## **Practical Questions**

### **Question 15: Go Program Structure**

```go
package main

import "fmt"

// Person represents a person with name and age
type Person struct {
    Name string
    Age  int
}

// PrintInfo prints a person's information
func (p Person) PrintInfo() {
    fmt.Printf("Name: %s, Age: %d\n", p.Name, p.Age)
}

func main() {
    // Create a new person
    person := Person{
        Name: "Alice",
        Age:  30,
    }
    
    // Print the person's information
    person.PrintInfo()
}
```

### **Question 16: Git Workflow**

The typical Git workflow for making changes:

1. **Check status**: `git status` - See current state
2. **Create branch**: `git checkout -b feature/new-feature` - Create feature branch
3. **Make changes**: Edit files in your code editor
4. **Stage changes**: `git add .` - Stage all changes
5. **Commit changes**: `git commit -m "Descriptive message"` - Commit with message
6. **Push changes**: `git push origin feature/new-feature` - Push to remote
7. **Create pull request**: On GitHub/GitLab, create PR to merge into main
8. **Merge**: After review, merge the feature branch into main
9. **Clean up**: `git checkout main && git pull && git branch -d feature/new-feature`

### **Question 17: Project Structure**

- **`cmd/`**: Contains the main entry points for applications. Each subdirectory represents a different executable (e.g., `cmd/server/`, `cmd/cli/`).

- **`internal/`**: Contains private application code that should not be imported by other projects. This is enforced by the Go compiler.

- **`sdk/`**: Contains the software development kit - public APIs and interfaces that can be used by other projects.

- **`docs/`**: Contains project documentation, API references, architecture guides, and other documentation.

### **Question 18: Blockchain Concepts**

Blocks in a blockchain are connected through cryptographic hashing:

1. **Previous Hash**: Each block contains the hash of the previous block in its header
2. **Chain Linking**: This creates a chain where each block is linked to the previous one
3. **Immutability**: If any block is modified, its hash changes, breaking the chain
4. **Verification**: The chain can be verified by recalculating hashes and checking the links
5. **Genesis Block**: The first block (genesis block) has no previous block, so it typically has a special previous hash value

This linking mechanism ensures data integrity and makes the blockchain tamper-evident.

---

## **Bonus Challenge**

### **Question 19: Enhanced Hello Blockchain**

```go
package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "time"
)

// Block represents a blockchain block
type Block struct {
    Index        int
    Timestamp    time.Time
    Data         string
    PreviousHash string
    Hash         string
}

// Blockchain represents a simple blockchain
type Blockchain struct {
    Blocks []*Block
}

// calculateHash calculates a simple hash for a block
func calculateHash(index int, timestamp time.Time, data, previousHash string) string {
    // Create a string representation of the block
    blockString := fmt.Sprintf("%d%s%s%s", index, timestamp.Format(time.RFC3339), data, previousHash)
    
    // Calculate SHA-256 hash
    hash := sha256.Sum256([]byte(blockString))
    return hex.EncodeToString(hash[:])
}

// NewBlock creates a new block
func NewBlock(index int, data, previousHash string) *Block {
    timestamp := time.Now()
    hash := calculateHash(index, timestamp, data, previousHash)
    
    return &Block{
        Index:        index,
        Timestamp:    timestamp,
        Data:         data,
        PreviousHash: previousHash,
        Hash:         hash,
    }
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(data string) {
    var previousHash string
    if len(bc.Blocks) > 0 {
        previousHash = bc.Blocks[len(bc.Blocks)-1].Hash
    } else {
        previousHash = "0" // Genesis block has no previous hash
    }
    
    index := len(bc.Blocks)
    block := NewBlock(index, data, previousHash)
    bc.Blocks = append(bc.Blocks, block)
}

// ValidateChain validates the integrity of the blockchain
func (bc *Blockchain) ValidateChain() bool {
    for i := 1; i < len(bc.Blocks); i++ {
        currentBlock := bc.Blocks[i]
        previousBlock := bc.Blocks[i-1]
        
        // Check if the previous hash matches
        if currentBlock.PreviousHash != previousBlock.Hash {
            return false
        }
        
        // Recalculate hash to ensure integrity
        calculatedHash := calculateHash(
            currentBlock.Index,
            currentBlock.Timestamp,
            currentBlock.Data,
            currentBlock.PreviousHash,
        )
        
        if currentBlock.Hash != calculatedHash {
            return false
        }
    }
    
    return true
}

// Display displays all blocks in the blockchain
func (bc *Blockchain) Display() {
    fmt.Println("=== Blockchain ===")
    for _, block := range bc.Blocks {
        fmt.Printf("Block #%d\n", block.Index)
        fmt.Printf("Timestamp: %s\n", block.Timestamp.Format(time.RFC3339))
        fmt.Printf("Data: %s\n", block.Data)
        fmt.Printf("Previous Hash: %s\n", block.PreviousHash)
        fmt.Printf("Hash: %s\n", block.Hash)
        fmt.Println("---")
    }
}

func main() {
    fmt.Println("🚀 Enhanced Hello Blockchain!")
    fmt.Println("Creating a linked blockchain...\n")

    // Create a new blockchain
    blockchain := &Blockchain{}

    // Add some blocks
    blockchain.AddBlock("Genesis Block - The beginning of our blockchain!")
    blockchain.AddBlock("Second Block - Learning Go and blockchain!")
    blockchain.AddBlock("Third Block - Building something amazing!")

    // Display the blockchain
    blockchain.Display()

    // Validate the chain
    if blockchain.ValidateChain() {
        fmt.Println("✅ Blockchain is valid!")
    } else {
        fmt.Println("❌ Blockchain is invalid!")
    }

    fmt.Println("\n🎉 Your enhanced blockchain is ready!")
    fmt.Println("Features implemented:")
    fmt.Println("- Cryptographic hashing with SHA-256")
    fmt.Println("- Block linking with previous hash")
    fmt.Println("- Chain validation")
    fmt.Println("- Immutable data structure")
}
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on code completeness and functionality

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have a strong understanding of the fundamentals
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 2
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 1! 🎉**

Ready for the next challenge? Move on to [Section 2: Go Fundamentals for Blockchain](../section2/README.md)!
