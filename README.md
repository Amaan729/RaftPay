# RaftPay

> **Distributed financial transaction ledger with Raft consensus - built from scratch in Go**

A production-ready distributed payment system implementing the Raft consensus algorithm for strong consistency and fault tolerance. Features persistent storage, automatic leader election, and sub-5ms p99 latency at 7,600+ TPS.

[![Go Version](https://img.shields.io/badge/Go-1.21-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-brightgreen.svg)](docker-compose.yml)

---

## ✨ Features

### **Distributed Consensus**
- ✅ **Raft consensus algorithm** - Leader election, log replication, safety guarantees
- ✅ **Strong consistency** - Linearizable reads and writes across all nodes
- ✅ **Fault tolerance** - Survives minority node failures (2 out of 5 nodes)
- ✅ **Automatic failover** - Sub-second leader re-election

### **Production Ready**
- ✅ **Persistent storage** - Crash recovery with write-ahead logging
- ✅ **REST API** - Clean HTTP interface with automatic leader redirection
- ✅ **Docker deployment** - Multi-node cluster with Docker Compose
- ✅ **High performance** - 7,600+ TPS with 4.68ms p99 latency

### **Financial Operations**
- ✅ **Account management** - Create accounts with initial balances
- ✅ **Idempotent transfers** - Exactly-once semantics with transaction IDs
- ✅ **ACID guarantees** - Atomic, consistent, isolated, durable transactions
- ✅ **Balance queries** - Real-time account balance lookups

---

## 🚀 Quick Start

### **Prerequisites**
- Docker & Docker Compose
- Go 1.21+ (for local development)

### **Run the 5-node cluster:**

```bash
# Clone the repository
git clone https://github.com/Amaan729/RaftPay.git
cd RaftPay

# Start the cluster
docker compose up -d

# Run the demo (creates accounts, makes transfers, tests failover)
./demo.sh
```

The cluster will start with 5 nodes on ports **9000-9004**.

### **Make your first transfer:**

```bash
# Find the leader
curl http://localhost:9000/status | jq '.state'

# Create accounts
curl -X POST http://localhost:9001/account \
  -H "Content-Type: application/json" \
  -d '{"account_id":"alice","initial_balance":1000.00}'

curl -X POST http://localhost:9001/account \
  -H "Content-Type: application/json" \
  -d '{"account_id":"bob","initial_balance":500.00}'

# Make a transfer
curl -X POST http://localhost:9001/transfer \
  -H "Content-Type: application/json" \
  -d '{"from":"alice","to":"bob","amount":100.00,"txn_id":"tx_001"}'

# Check balances
curl http://localhost:9001/account/alice | jq '.balance'
# Output: 900.00

curl http://localhost:9001/account/bob | jq '.balance'
# Output: 600.00
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│              Docker Network                      │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │  node1   │  │  node2   │  │  node3   │      │
│  │ Leader   │  │ Follower │  │ Follower │      │
│  │ :9000    │  │ :9001    │  │ :9002    │      │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘      │
│       │             │             │             │
│       └─────────────┼─────────────┘             │
│              Raft Consensus                     │
│       ┌─────────────┼─────────────┐             │
│       │             │             │             │
│  ┌────┴─────┐  ┌───┴──────┐                    │
│  │  node4   │  │  node5   │                    │
│  │ Follower │  │ Follower │                    │
│  │ :9003    │  │ :9004    │                    │
│  └──────────┘  └──────────┘                    │
│                                                  │
│  Persistent Volumes (Crash Recovery)            │
└─────────────────────────────────────────────────┘
```

### **Key Components:**

**Raft Consensus Layer** (`raft/`)
- Leader election with randomized timeouts
- Log replication with consistency checks
- Commit index advancement via quorum
- Persistent state (term, votedFor, log)

**Financial Ledger** (`ledger/`)
- Account management with balance tracking
- Idempotent transfer handling
- State machine replication from Raft log

**REST API** (`api/`)
- HTTP server with JSON endpoints
- Automatic leader redirection (307)
- Status monitoring and cluster information

**Persistence Layer** (`raft/persistence.go`)
- Write-ahead logging for durability
- Crash recovery with state restoration
- Buffered writes for performance

---

## 📊 Performance

Benchmark results from local testing (5-node cluster):

```
Total Transfers:  5,000
Successful:       5,000 (100%)
Failed:           0 (0%)
Duration:         0.65 seconds
Throughput:       7,663 transfers/sec

Latency Distribution:
  Minimum:        0.20 ms
  p50 (median):   1.06 ms
  p95:            2.99 ms
  p99:            4.68 ms
  Maximum:        11.02 ms
  Average:        1.29 ms
  Std Dev:        0.85 ms
```

**Key Insights:**
- **7,600+ TPS** - Production-grade throughput
- **Sub-5ms p99 latency** - Excellent tail latency
- **100% success rate** - Zero failures under load
- **Strong consistency** - All reads reflect committed writes

---

## 🔌 API Reference

### **Status Endpoint**
Get node state and cluster information.

```bash
GET /status
```

**Response:**
```json
{
  "node_id": "node1",
  "state": "LEADER",
  "term": 5,
  "commit_index": 42,
  "last_applied": 42,
  "log_length": 43,
  "peers": ["node2", "node3", "node4", "node5"]
}
```

### **Create Account**
Create a new account with an initial balance.

```bash
POST /account
Content-Type: application/json

{
  "account_id": "alice",
  "initial_balance": 1000.00
}
```

**Response:**
```json
{
  "success": true,
  "message": "Account created successfully",
  "account_id": "alice",
  "balance": 1000.00
}
```

### **Get Account Balance**
Query an account's current balance.

```bash
GET /account/{account_id}
```

**Response:**
```json
{
  "account_id": "alice",
  "balance": 1000.00
}
```

### **Make Transfer**
Transfer funds between accounts (idempotent).

```bash
POST /transfer
Content-Type: application/json

{
  "from": "alice",
  "to": "bob",
  "amount": 100.00,
  "txn_id": "unique_transaction_id"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Transfer completed successfully",
  "from": "alice",
  "to": "bob",
  "amount": 100.00,
  "txn_id": "unique_transaction_id"
}
```

**Idempotency:** Using the same `txn_id` twice will return success without double-spending.

---

## 🧪 Testing

### **Run Unit Tests**
```bash
go test ./... -v
```

### **Run Benchmark**
```bash
go test -v -run TestBenchmarkTransfers -timeout 5m
```

### **Chaos Testing**
The demo script includes automated chaos testing:
1. Starts 5-node cluster
2. Creates accounts and makes transfers
3. **Kills the leader node**
4. Verifies automatic failover
5. Confirms data integrity after recovery

```bash
./demo.sh
```

---

## 🛠️ Development

### **Project Structure**
```
RaftPay/
├── main.go              # Application entry point
├── go.mod               # Go module dependencies
├── benchmark_test.go    # Performance benchmarks
├── demo.sh              # Automated demo script
├── README.md            # Project documentation
├── ARCHITECTURE.md      # Detailed technical documentation
├── Dockerfile           # Container image definition
├── docker-compose.yml   # Multi-node cluster orchestration
├── .dockerignore        # Docker build exclusions
│
├── raft/                # Raft consensus implementation
│   ├── node.go          # Core node state and logic
│   ├── election.go      # Leader election algorithm
│   ├── replication.go   # Log replication mechanism
│   ├── rpc.go           # RPC message definitions
│   ├── http_server.go   # HTTP transport layer
│   ├── persistence.go   # Persistent storage (WAL)
│   ├── config.go        # Configuration management
│   ├── utils.go         # Utility functions
│   ├── state_machine.go # State machine interface
│   ├── transport.go     # Network transport abstraction
│   ├── proposal.go      # Client proposal handling
│   └── types.go         # Type definitions
│
├── ledger/              # Financial ledger layer
│   ├── ledger.go        # Core ledger logic
│   └── raft_ledger.go   # Raft-integrated ledger
│
└── api/                 # REST API layer
    ├── handlers.go      # HTTP request handlers
    └── types.go         # API type definitions
```

### **Build from Source**
```bash
# Install dependencies
go mod download

# Build binary
go build -o raftpay .

# Run a single node
./raftpay --id=node1 --peers=node2,node3 --raft-port=8000 --api-port=9000
```

---

## 🐛 Debugging

### **View Node Logs**
```bash
# All nodes
docker compose logs -f

# Specific node
docker compose logs -f node1

# Last 50 lines
docker compose logs --tail=50 node3
```

### **Check Cluster Health**
```bash
# Check all nodes
for port in 9000 9001 9002 9003 9004; do
  echo "Node on port $port:"
  curl -s http://localhost:$port/status | jq '{node_id, state, term}'
done
```

### **Common Issues**

**No leader elected:**
- Wait 15-20 seconds after startup
- Check logs for election messages
- Ensure all nodes can communicate

**Transfer fails:**
- Verify you're sending to the leader node
- Check account exists with sufficient balance
- Ensure `txn_id` is unique

---

## 🎓 Learning Resources

### **Raft Consensus**
- [The Raft Paper](https://raft.github.io/raft.pdf) - Original research paper
- [Raft Visualization](https://raft.github.io/) - Interactive demo
- [MIT 6.824](https://pdos.csail.mit.edu/6.824/) - Distributed Systems course

### **Implementation Notes**
- See [ARCHITECTURE.md](ARCHITECTURE.md) for deep technical dive
- Election timeout: 300-600ms (configurable)
- Heartbeat interval: 75ms
- Persistent storage with buffered writes

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### **Development Setup**
1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- **Diego Ongaro & John Ousterhout** - For the Raft consensus algorithm
- **MIT 6.824** - Distributed Systems course materials
- **Go community** - For excellent standard library and tooling

---

## 📬 Contact

**Amaan Sayed**
- GitHub: [@Amaan729](https://github.com/Amaan729)
- Project: [github.com/Amaan729/RaftPay](https://github.com/Amaan729/RaftPay)

---

**Built with ❤️ using Go and Raft**
