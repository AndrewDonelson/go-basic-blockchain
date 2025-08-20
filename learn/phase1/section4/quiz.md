# Section 4 Quiz: Core Data Structures

## 📝 Test Your Knowledge

This quiz will test your understanding of core data structures essential for blockchain development. Take your time and think through each question carefully.

---

## **Multiple Choice Questions**

### **Question 1: Block Structure**
What is the primary purpose of the Merkle root in a block header?

A) To store transaction data  
B) To efficiently verify transaction inclusion  
C) To link blocks together  
D) To store block metadata

### **Question 2: Transaction Interface**
What is the main benefit of using interfaces for transactions in Go?

A) They provide inheritance like in object-oriented languages  
B) They allow for polymorphism and different transaction types  
C) They improve performance by reducing memory usage  
D) They enable multiple inheritance

### **Question 3: Wallet System**
What is the primary purpose of a wallet in blockchain systems?

A) To store cryptocurrency  
B) To manage cryptographic keys and addresses  
C) To process transactions  
D) To mine blocks

### **Question 4: PUID System**
What does PUID stand for in blockchain development?

A) Persistent Unique Identifier  
B) Public User ID  
C) Private Unique ID  
D) Permanent User Identifier

### **Question 5: Merkle Trees**
How do Merkle trees improve blockchain efficiency?

A) They reduce block size  
B) They allow efficient verification of transaction inclusion  
C) They speed up mining  
D) They compress transaction data

### **Question 6: JSON Tags**
What does the `omitempty` tag do in Go struct JSON tags?

A) It makes the field required  
B) It omits the field from JSON output if it's empty  
C) It encrypts the field value  
D) It makes the field optional

### **Question 7: Block Validation**
What is the first step in validating a block?

A) Check the proof of work  
B) Verify the block hash  
C) Validate all transactions  
D) Check the block index

### **Question 8: Address Generation**
What cryptographic function is typically used to generate wallet addresses?

A) SHA-256  
B) RIPEMD-160  
C) Both SHA-256 and RIPEMD-160  
D) MD5

---

## **True/False Questions**

### **Question 9**
A block header contains all the metadata needed to identify and validate a block.

**True** / **False**

### **Question 10**
Transaction interfaces in Go must be explicitly implemented by declaring that a type implements the interface.

**True** / **False**

### **Question 11**
Merkle trees allow you to verify that a transaction is included in a block without downloading the entire block.

**True** / **False**

### **Question 12**
Wallet private keys should be stored in plain text for easy access.

**True** / **False**

### **Question 13**
PUIDs are used to provide unique identifiers that persist across blockchain operations.

**True** / **False**

### **Question 14**
Block size calculation is important for network transmission and storage efficiency.

**True** / **False**

---

## **Practical Questions**

### **Question 15: Block Structure Implementation**
Create a complete block structure with header and body separation. Include all necessary fields and implement the hash calculation method.

### **Question 16: Transaction Interface Design**
Design a transaction interface and implement two different transaction types (Bank and Message) that satisfy the interface.

### **Question 17: Wallet System Implementation**
Implement a wallet system with key generation, address derivation, and balance tracking functionality.

### **Question 18: Merkle Tree Implementation**
Create a function that builds a Merkle tree from a list of transaction hashes and returns the root hash.

---

## **Bonus Challenge**

### **Question 19: Complete Data Structure System**
Create a complete data structure system that includes:
1. Enhanced block structure with Merkle tree
2. Multiple transaction types with interface implementation
3. Wallet system with key management
4. PUID system for unique identifiers
5. JSON serialization/deserialization
6. Comprehensive validation methods
7. A simple test to demonstrate functionality

Write the complete implementation with proper error handling and documentation.

---

## **Scoring Guide**

- **Multiple Choice (8 questions)**: 2 points each = 16 points
- **True/False (6 questions)**: 1 point each = 6 points
- **Practical Questions (4 questions)**: 5 points each = 20 points
- **Bonus Challenge**: 10 points

**Total Possible Score: 52 points**

### **Passing Grades:**
- **Excellent (90%+)**: 47+ points
- **Good (80-89%)**: 42-46 points
- **Satisfactory (70-79%)**: 36-41 points
- **Needs Improvement (<70%)**: <36 points

---

## **Quiz Instructions**

1. Answer all questions to the best of your ability
2. For practical questions, provide clear, well-structured code
3. For the bonus challenge, write complete, runnable code
4. Review your answers before submitting
5. Check the [answers](./answers.md) after completing the quiz

---

**Good luck! 🚀**

When you're ready, check your answers against the [Section 4 Quiz Answers](./answers.md).
