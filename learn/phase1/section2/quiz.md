# Section 2 Quiz: Go Fundamentals for Blockchain

## 📝 Test Your Knowledge

This quiz will test your understanding of Go fundamentals essential for blockchain development. Take your time and think through each question carefully.

---

## **Multiple Choice Questions**

### **Question 1: Go Structs**
In Go, what is the correct way to define a method on a struct?

A) `func methodName(StructName) returnType { }`  
B) `func (s StructName) methodName() returnType { }`  
C) `func (s *StructName) methodName() returnType { }`  
D) `func StructName.methodName() returnType { }`

### **Question 2: Interfaces**
What is the main benefit of using interfaces in Go?

A) They provide inheritance like in object-oriented languages  
B) They allow for polymorphism and code flexibility  
C) They improve performance by reducing memory usage  
D) They enable multiple inheritance

### **Question 3: Goroutines**
How do you start a goroutine in Go?

A) `go functionName()`  
B) `goroutine functionName()`  
C) `async functionName()`  
D) `thread functionName()`

### **Question 4: Channels**
What is the default capacity of an unbuffered channel in Go?

A) 0  
B) 1  
C) 10  
D) Unlimited

### **Question 5: Cryptographic Hashing**
Which Go package provides SHA-256 hashing functionality?

A) `crypto/hash`  
B) `crypto/sha256`  
C) `hash/sha256`  
D) `crypto/encoding`

### **Question 6: JSON Tags**
What does the `omitempty` tag do in JSON struct tags?

A) It makes the field required  
B) It omits the field from JSON output if it's empty  
C) It encrypts the field value  
D) It makes the field optional

### **Question 7: Error Handling**
What is the idiomatic way to handle errors in Go?

A) Using try-catch blocks  
B) Returning error values and checking them  
C) Using panic and recover  
D) Ignoring errors

### **Question 8: File I/O**
Which function is used to read an entire file into memory in Go?

A) `os.ReadFile()`  
B) `ioutil.ReadFile()`  
C) `file.ReadAll()`  
D) All of the above

---

## **True/False Questions**

### **Question 9**
In Go, interfaces are implemented implicitly - you don't need to explicitly declare that a type implements an interface.

**True** / **False**

### **Question 10**
Goroutines are lightweight threads managed by the Go runtime, not the operating system.

**True** / **False**

### **Question 11**
Channels in Go can only be used to communicate between goroutines.

**True** / **False**

### **Question 12**
The `crypto/rand` package provides cryptographically secure random number generation.

**True** / **False**

### **Question 13**
In Go, you can use `json.Marshal()` to convert a struct to JSON, but you need to use `json.Unmarshal()` to convert JSON back to a struct.

**True** / **False**

### **Question 14**
Custom error types in Go should implement the `error` interface by defining an `Error() string` method.

**True** / **False**

---

## **Practical Questions**

### **Question 15: Struct and Interface Implementation**
Create a simple `Wallet` struct and a `Transaction` interface. The wallet should have fields for address and balance, and implement methods to send and receive transactions.

### **Question 16: Goroutine and Channel Usage**
Write a simple program that uses a goroutine to process transactions and a channel to communicate between the main function and the goroutine.

### **Question 17: Cryptographic Operations**
Implement a function that generates a SHA-256 hash of a given string and another function that generates a random wallet address.

### **Question 18: JSON Serialization**
Create a `Block` struct with JSON tags and implement functions to serialize it to JSON and deserialize it from JSON.

---

## **Bonus Challenge**

### **Question 19: Complete Blockchain Component**
Create a simple blockchain component that includes:
1. A `Block` struct with proper JSON tags
2. A `Blockchain` struct that manages blocks
3. Methods to add blocks and validate the chain
4. JSON serialization/deserialization
5. Basic error handling
6. A simple test to verify functionality

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

When you're ready, check your answers against the [Section 2 Quiz Answers](./answers.md).
