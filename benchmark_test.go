package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Amaan729/RaftPay/api"
	"github.com/Amaan729/RaftPay/ledger"
	"github.com/Amaan729/RaftPay/raft"
)

// BenchmarkTransfers measures system performance under load
func TestBenchmarkTransfers(t *testing.T) {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🚀 RAFTPAY PERFORMANCE BENCHMARK")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	
	// ========== PHASE 1: CLUSTER SETUP ==========
	fmt.Println("📋 Phase 1: Setting up 3-node cluster...")
	
	// Setup nodes (same as test_api.go but silent)
	applyCh1 := make(chan raft.ApplyMsg, 100)
	applyCh2 := make(chan raft.ApplyMsg, 100)
	applyCh3 := make(chan raft.ApplyMsg, 100)
	
	// Disable verbose Raft logging for benchmark
	log.SetOutput(io.Discard)
	
	// Node 1
	node1 := raft.NewRaftNode("bench_node1", []string{"bench_node2", "bench_node3"}, applyCh1)
	peerURLs1 := map[string]string{
		"bench_node2": "http://localhost:8010",
		"bench_node3": "http://localhost:8011",
	}
	transport1 := raft.NewTransport(node1, "http://localhost:8009", peerURLs1)
	node1.SetTransport(transport1)
	
	server1 := raft.NewHTTPServer(node1)
	go server1.StartServer(":8009")
	
	raftLedger1 := ledger.NewRaftLedger(node1, applyCh1)
	raftLedger1.Start()
	
	apiServer1 := api.NewAPIServer(node1, raftLedger1, "bench_node1", ":9009")
	go apiServer1.StartServer()
	
	node1.Start()
	
	// Node 2
	node2 := raft.NewRaftNode("bench_node2", []string{"bench_node1", "bench_node3"}, applyCh2)
	peerURLs2 := map[string]string{
		"bench_node1": "http://localhost:8009",
		"bench_node3": "http://localhost:8011",
	}
	transport2 := raft.NewTransport(node2, "http://localhost:8010", peerURLs2)
	node2.SetTransport(transport2)
	
	server2 := raft.NewHTTPServer(node2)
	go server2.StartServer(":8010")
	
	raftLedger2 := ledger.NewRaftLedger(node2, applyCh2)
	raftLedger2.Start()
	
	apiServer2 := api.NewAPIServer(node2, raftLedger2, "bench_node2", ":9010")
	go apiServer2.StartServer()
	
	node2.Start()
	
	// Node 3
	node3 := raft.NewRaftNode("bench_node3", []string{"bench_node1", "bench_node2"}, applyCh3)
	peerURLs3 := map[string]string{
		"bench_node1": "http://localhost:8009",
		"bench_node2": "http://localhost:8010",
	}
	transport3 := raft.NewTransport(node3, "http://localhost:8011", peerURLs3)
	node3.SetTransport(transport3)
	
	server3 := raft.NewHTTPServer(node3)
	go server3.StartServer(":8011")
	
	raftLedger3 := ledger.NewRaftLedger(node3, applyCh3)
	raftLedger3.Start()
	
	apiServer3 := api.NewAPIServer(node3, raftLedger3, "bench_node3", ":9011")
	go apiServer3.StartServer()
	
	node3.Start()
	
	// Wait for startup
	time.Sleep(200 * time.Millisecond)
	
	// Re-enable logging for benchmark results
	log.SetOutput(os.Stderr)
	
	fmt.Println("✅ Cluster ready!")
	fmt.Println()
	
	// ========== PHASE 2: WAIT FOR LEADER ==========
	fmt.Println("📋 Phase 2: Waiting for leader election...")
	
	var leaderPort string
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		
		_, isLeader1 := node1.GetState()
		_, isLeader2 := node2.GetState()
		_, isLeader3 := node3.GetState()
		
		if isLeader1 {
			leaderPort = "9009"
			break
		} else if isLeader2 {
			leaderPort = "9010"
			break
		} else if isLeader3 {
			leaderPort = "9011"
			break
		}
	}
	
	if leaderPort == "" {
		t.Fatal("❌ No leader elected!")
	}
	
	fmt.Printf("✅ Leader elected on port %s\n", leaderPort)
	fmt.Println()
	
	baseURL := "http://localhost:" + leaderPort
	
	// ========== PHASE 3: CREATE TEST ACCOUNTS ==========
	fmt.Println("📋 Phase 3: Creating test accounts...")
	
	// Create 100 accounts with $1,000,000 each
	numAccounts := 100
	for i := 0; i < numAccounts; i++ {
		accountID := fmt.Sprintf("bench_account_%d", i)
		benchCreateAccount(baseURL, accountID, 1000000.0)
	}
	
	// Wait for accounts to be created
	time.Sleep(1 * time.Second)
	
	fmt.Printf("✅ Created %d accounts\n", numAccounts)
	fmt.Println()
	
	// ========== PHASE 4: WARMUP ==========
	fmt.Println("📋 Phase 4: Warming up (100 transfers)...")
	
	for i := 0; i < 100; i++ {
		from := fmt.Sprintf("bench_account_%d", i%numAccounts)
		to := fmt.Sprintf("bench_account_%d", (i+1)%numAccounts)
		benchTransfer(baseURL, from, to, 1.0, fmt.Sprintf("warmup_%d", i))
	}
	
	time.Sleep(500 * time.Millisecond)
	fmt.Println("✅ Warmup complete")
	fmt.Println()
	
	// ========== PHASE 5: BENCHMARK - CONCURRENT TRANSFERS ==========
	fmt.Println("📋 Phase 5: Running benchmark (10,000 concurrent transfers)...")
	fmt.Println()
	
	numTransfers := 10000
	concurrency := 100 // How many transfers to run in parallel
	
	// Track latencies for each transfer
	latencies := make([]float64, numTransfers)
	var latencyMutex sync.Mutex
	
	// Track successes and failures
	var successCount, failCount int32
	var countMutex sync.Mutex
	
	// Start timer
	benchmarkStart := time.Now()
	
	// Semaphore to limit concurrency
	sem := make(chan struct{}, concurrency)
	
	// WaitGroup to wait for all transfers
	var wg sync.WaitGroup
	
	// Launch all transfers
	for i := 0; i < numTransfers; i++ {
		wg.Add(1)
		
		go func(idx int) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()
			
			// Pick accounts to transfer between
			from := fmt.Sprintf("bench_account_%d", idx%numAccounts)
			to := fmt.Sprintf("bench_account_%d", (idx+1)%numAccounts)
			txnID := fmt.Sprintf("bench_txn_%d", idx)
			
			// Measure latency of this transfer
			start := time.Now()
			err := benchTransferWithError(baseURL, from, to, 0.01, txnID)
			duration := time.Since(start)
			
			// Record latency in milliseconds
			latencyMutex.Lock()
			latencies[idx] = float64(duration.Microseconds()) / 1000.0
			latencyMutex.Unlock()
			
			// Track success/failure
			countMutex.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			countMutex.Unlock()
			
			// Print progress every 1000 transfers
			if (idx+1)%1000 == 0 {
				elapsed := time.Since(benchmarkStart).Seconds()
				currentThroughput := float64(idx+1) / elapsed
				fmt.Printf("  Progress: %d/%d transfers (%.0f TPS)\n", idx+1, numTransfers, currentThroughput)
			}
		}(i)
	}
	
	// Wait for all transfers to complete
	wg.Wait()
	
	// Stop timer
	benchmarkDuration := time.Since(benchmarkStart)
	
	fmt.Println()
	fmt.Println("✅ Benchmark complete!")
	fmt.Println()
	
	// ========== PHASE 6: CALCULATE STATISTICS ==========
	fmt.Println("📋 Phase 6: Calculating statistics...")
	fmt.Println()
	
	// Sort latencies to calculate percentiles
	sort.Float64s(latencies)
	
	// Calculate percentiles
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	
	// Calculate min/max/avg
	minLatency := latencies[0]
	maxLatency := latencies[len(latencies)-1]
	
	var sumLatency float64
	for _, lat := range latencies {
		sumLatency += lat
	}
	avgLatency := sumLatency / float64(len(latencies))
	
	// Calculate standard deviation
	var variance float64
	for _, lat := range latencies {
		variance += math.Pow(lat-avgLatency, 2)
	}
	stdDev := math.Sqrt(variance / float64(len(latencies)))
	
	// Calculate throughput
	throughput := float64(successCount) / benchmarkDuration.Seconds()
	
	// ========== PHASE 7: PRINT RESULTS ==========
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("📊 BENCHMARK RESULTS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	
	fmt.Println("⏱️  THROUGHPUT:")
	fmt.Printf("  Total Transfers:  %d\n", numTransfers)
	fmt.Printf("  Successful:       %d (%.1f%%)\n", successCount, float64(successCount)/float64(numTransfers)*100)
	fmt.Printf("  Failed:           %d (%.1f%%)\n", failCount, float64(failCount)/float64(numTransfers)*100)
	fmt.Printf("  Duration:         %.2f seconds\n", benchmarkDuration.Seconds())
	fmt.Printf("  Throughput:       %.0f transfers/sec\n", throughput)
	fmt.Println()
	
	fmt.Println("📈 LATENCY DISTRIBUTION (milliseconds):")
	fmt.Printf("  Minimum:      %.2f ms\n", minLatency)
	fmt.Printf("  p50 (median): %.2f ms\n", p50)
	fmt.Printf("  p95:          %.2f ms\n", p95)
	fmt.Printf("  p99:          %.2f ms\n", p99)
	fmt.Printf("  Maximum:      %.2f ms\n", maxLatency)
	fmt.Printf("  Average:      %.2f ms\n", avgLatency)
	fmt.Printf("  Std Dev:      %.2f ms\n", stdDev)
	fmt.Println()
	
	// Visual histogram
	fmt.Println("📊 LATENCY HISTOGRAM:")
	benchPrintHistogram(latencies)
	fmt.Println()
	
	// ========== PHASE 8: PASS/FAIL CRITERIA ==========
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("✅ PERFORMANCE GOALS:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	
	// Goal 1: Throughput > 1000 TPS
	if throughput >= 1000 {
		fmt.Printf("  ✅ Throughput: %.0f TPS (goal: 1000+ TPS)\n", throughput)
	} else {
		fmt.Printf("  ❌ Throughput: %.0f TPS (goal: 1000+ TPS)\n", throughput)
	}
	
	// Goal 2: p99 latency < 50ms
	if p99 < 50 {
		fmt.Printf("  ✅ p99 Latency: %.2f ms (goal: <50ms)\n", p99)
	} else {
		fmt.Printf("  ❌ p99 Latency: %.2f ms (goal: <50ms)\n", p99)
	}
	
	// Goal 3: Success rate > 99%
	successRate := float64(successCount) / float64(numTransfers) * 100
	if successRate >= 99 {
		fmt.Printf("  ✅ Success Rate: %.1f%% (goal: >99%%)\n", successRate)
	} else {
		fmt.Printf("  ❌ Success Rate: %.1f%% (goal: >99%%)\n", successRate)
	}
	
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	
	// Cleanup
	defer node1.Stop()
	defer node2.Stop()
	defer node3.Stop()
}

// Helper: Create account via API (renamed to avoid conflict)
func benchCreateAccount(baseURL, accountID string, balance float64) error {
	req := api.CreateAccountRequest{
		AccountID:      accountID,
		InitialBalance: balance,
	}
	
	body, _ := json.Marshal(req)
	_, err := http.Post(baseURL+"/account", "application/json", bytes.NewBuffer(body))
	return err
}

// Helper: Submit transfer via API (renamed to avoid conflict)
func benchTransfer(baseURL, from, to string, amount float64, txnID string) error {
	return benchTransferWithError(baseURL, from, to, amount, txnID)
}

// Helper: Submit transfer and return error (renamed to avoid conflict)
func benchTransferWithError(baseURL, from, to string, amount float64, txnID string) error {
	req := api.TransferRequest{
		From:   from,
		To:     to,
		Amount: amount,
		TxnID:  txnID,
	}
	
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/transfer", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	return nil
}

// Helper: Print ASCII histogram of latencies (renamed to avoid conflict)
func benchPrintHistogram(latencies []float64) {
	// Create 10 buckets
	numBuckets := 10
	buckets := make([]int, numBuckets)
	
	minLat := latencies[0]
	maxLat := latencies[len(latencies)-1]
	bucketSize := (maxLat - minLat) / float64(numBuckets)
	
	// Fill buckets
	for _, lat := range latencies {
		bucketIdx := int((lat - minLat) / bucketSize)
		if bucketIdx >= numBuckets {
			bucketIdx = numBuckets - 1
		}
		buckets[bucketIdx]++
	}
	
	// Find max bucket for scaling
	maxCount := 0
	for _, count := range buckets {
		if count > maxCount {
			maxCount = count
		}
	}
	
	// Print histogram
	barWidth := 50
	for i, count := range buckets {
		rangeStart := minLat + float64(i)*bucketSize
		rangeEnd := rangeStart + bucketSize
		
		barLen := (count * barWidth) / maxCount
		if barLen == 0 && count > 0 {
			barLen = 1
		}
		
		bar := strings.Repeat("█", barLen)
		fmt.Printf("  %.1f-%.1f ms: %s %d\n", rangeStart, rangeEnd, bar, count)
	}
}
