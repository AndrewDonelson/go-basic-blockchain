# Section 5 Quiz: Basic Blockchain Implementation

## 📝 Test Your Knowledge

This quiz will test your understanding of basic blockchain implementation and working blockchain systems. Take your time and think through each question carefully.

---

## **Multiple Choice Questions**

### **Question 1: Genesis Block**
What is the primary purpose of a genesis block in a blockchain?

A) To store the most recent transactions  
B) To serve as the first block and establish the initial state  
C) To contain mining rewards  
D) To validate the network

### **Question 2: Block Mining**
What is the main purpose of the nonce in block mining?

A) To identify the block  
B) To adjust the block hash to meet difficulty requirements  
C) To store transaction data  
D) To link blocks together

### **Question 3: Transaction Queue**
What happens to the transaction queue after a block is successfully mined?

A) It remains unchanged  
B) It is cleared and new transactions are added  
C) It is archived for historical purposes  
D) It is split into multiple queues

### **Question 4: Chain Validation**
What is the first step in validating a blockchain?

A) Check all transaction signatures  
B) Verify the genesis block  
C) Ensure blocks are linked by previous hash  
D) Validate proof of work

### **Question 5: Data Persistence**
What is the primary benefit of saving blockchain data to disk?

A) Faster transaction processing  
B) Data persistence across program restarts  
C) Reduced memory usage  
D) Better network performance

### **Question 6: Wallet Balance**
How is a wallet's balance typically calculated in a blockchain?

A) By summing all incoming transactions  
B) By tracking UTXOs (Unspent Transaction Outputs)  
C) By storing a balance field in the wallet  
D) By counting all transactions

### **Question 7: Mining Difficulty**
What happens to mining difficulty when blocks are mined too quickly?

A) It decreases to slow down mining  
B) It increases to speed up mining  
C) It increases to slow down mining  
D) It remains constant

### **Question 8: Block Time**
What is the purpose of setting a target block time in a blockchain?

A) To limit transaction processing  
B) To maintain consistent block creation rate  
C) To reduce energy consumption  
D) To increase transaction fees

---

## **True/False Questions**

### **Question 9**
A blockchain must have at least one transaction in each block to be valid.

**True** / **False**

### **Question 10**
The genesis block has a special previous hash value (usually empty or all zeros).

**True** / **False**

### **Question 11**
Block mining is a deterministic process that always produces the same result.

**True** / **False**

### **Question 12**
Wallet balances are updated immediately when transactions are added to the queue.

**True** / **False**

### **Question 13**
Blockchain data should be saved after every block is added to ensure data integrity.

**True** / **False**

### **Question 14**
The difficulty of mining a block is determined by the number of leading zeros required in the hash.

**True** / **False**

---

## **Practical Questions**

### **Question 15: Genesis Block Creation**
Create a function that generates a genesis block with initial transactions and proper validation.

### **Question 16: Block Mining Implementation**
Implement a block mining function that uses proof-of-work to find a valid nonce for a given difficulty.

### **Question 17: Transaction Queue Management**
Create a system to manage a transaction queue with proper validation and processing.

### **Question 18: Blockchain Persistence**
Implement functions to save and load a blockchain to/from disk with proper error handling.

---

## **Bonus Challenge**

### **Question 19: Complete Working Blockchain**
Create a complete, working blockchain system that includes:
1. Genesis block creation
2. Block mining with proof-of-work
3. Transaction queue management
4. Wallet balance tracking
5. Chain validation
6. Data persistence
7. Interactive command-line interface
8. A comprehensive test suite

Write the complete implementation with proper error handling, documentation, and a demonstration of all functionality.

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

When you're ready, check your answers against the [Section 5 Quiz Answers](./answers.md).
