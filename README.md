# go-basic-blockchain
A Basic Blockchain written from scratch and not using any 3rd party blockchain (go-ethereum, bitcoin, etc) to teach others.

## How to run

1. Clone the repo
2. Run `go mod tidy`
3. Run `go run main.go` in the root directory

## What is currently included

- [x] Basic Blockchain
- [ ] Proof of Work
- [x] Transactions
- [x] Wallets
- [ ] Mining Rewards
- [ ] Network
- [ ] Consensus
- [x] Persistence
- [ ] CLI
- [ ] Web Interface
- [x] Makefile (build, run, test, etc)
- [ ] Docker
- [x] Tests
- [ ] Documentation

## What is a Blockchain

A blockchain is a growing list of records, called blocks, that are linked together using cryptography. Each block contains a cryptographic hash of the previous block, a timestamp, and transaction data (generally represented as a Merkle tree).

By design, a blockchain is resistant to modification of its data. This is because once recorded, the data in any given block cannot be altered retroactively without alteration of all subsequent blocks. For use as a distributed ledger, a blockchain is typically managed by a peer-to-peer network collectively adhering to a protocol for inter-node communication and validating new blocks. Once recorded, the data in any given block cannot be altered retroactively without alteration of all subsequent blocks, which requires consensus of the network majority. Although blockchain records are not unalterable, blockchains may be considered secure by design and exemplify a distributed computing system with high Byzantine fault tolerance. Decentralized consensus has therefore been claimed with a blockchain.

## What is a Block

A block is a container data structure that aggregates transactions for inclusion in the public ledger, the blockchain. The block is made up of a header, containing metadata, followed by a long list of transactions that make up the bulk of its size.

## What is a Transaction

A transaction is a transfer of Bitcoin value that is broadcast to the network and collected into blocks. A transaction typically references previous transaction outputs as new transaction inputs and dedicates all input Bitcoin values to new outputs. Transactions are not encrypted, so it is possible to browse and view every transaction ever collected into a block. Once transactions are buried under enough confirmations they can be considered irreversible.

## What is a Wallet

A wallet is a collection of private keys that correspond to addresses. A private key is a secret number that allows Bitcoins to be spent. If a wallet’s private key is lost, the wallet loses its money. A wallet’s private keys are secret codes. Only the owner of the private key can send cryptocurrency. With no private key, a wallet cannot spend cryptocurrency. Therefore, it is very important to keep the private key safe.

## What is a Mining Reward

A mining reward is the amount of new cryptocurrency that is awarded to the miner of a block. It is part of the consensus algorithm in blockchains and is the incentive that miners have to mine on a given blockchain. The reward for mining a block is currently 6.25 Bitcoin.

## What is a Network

A network is a group of computers that are connected to each other for the purpose of communication. Networks may be classified according to a wide variety of characteristics. This article provides a general overview of types and categories and also presents the basic components of a network.

## What is Consensus

Consensus is a process that is used to achieve agreement on a blockchain network. Consensus enables network participants to agree on the contents of the blockchain in a distributed and trust-less manner. ... Consensus is reached through a majority vote of the network participants.

## What is Persistence

Persistence is the continuance of an effect after its cause is removed. In the context of blockchain, persistence is the ability to store data in a permanent state. This is achieved by storing data on a hard drive or other non-volatile storage medium.

## What is a CLI

A CLI is a command-line interface (CLI) processes commands to a computer program in the form of lines of text. The program which handles the interface is called a command-line interpreter or command-line processor. Operating systems implement a command-line interface in a shell for interactive access to operating system functions or services. Such access was primarily provided to users by computer terminals starting in the mid-1960s, and continued to be used throughout the 1970s and 1980s on VAX/VMS, Unix systems and personal computer systems including DOS, CP/M and Apple DOS.

## What is a Web Interface

A web interface is a system that allows users to interact with a web server from within their browser. Web interfaces have become increasingly common over the last decade as web browsers have become more powerful and the number of people with access to the internet has increased.

## What is Docker

Docker is a set of platform as a service (PaaS) products that use OS-level virtualization to deliver software in packages called containers. Containers are isolated from one another and bundle their own software, libraries and configuration files; they can communicate with each other through well-defined channels. All containers are run by a single operating system kernel and are thus more lightweight than virtual machines. Containers are created from images that specify their precise contents. Images are often created by combining and modifying standard images downloaded from public repositories.

## What is Testing

Testing is the process of evaluating a system or its component(s) with the intent to find whether it satisfies the specified requirements or not. In simple words, testing is executing a system in order to identify any gaps, errors, or missing requirements in contrary to the actual requirements.
