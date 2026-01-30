package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Amaan729/RaftPay/api"
	"github.com/Amaan729/RaftPay/ledger"
	"github.com/Amaan729/RaftPay/raft"
)

func main() {
	// Command-line flags
	nodeID := flag.String("id", "", "Node ID (e.g., node1)")
	peers := flag.String("peers", "", "Comma-separated peer IDs (e.g., node2,node3)")
	raftPort := flag.String("raft-port", "8000", "Raft RPC port")
	apiPort := flag.String("api-port", "9000", "REST API port")
	flag.Parse()

	// Validate required flags
	if *nodeID == "" {
		log.Fatal("❌ --id is required")
	}

	// Parse peers
	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
	}

	log.Printf("🚀 Starting RaftPay Node")
	log.Printf("   Node ID:    %s", *nodeID)
	log.Printf("   Peers:      %v", peerList)
	log.Printf("   Raft Port:  %s", *raftPort)
	log.Printf("   API Port:   %s", *apiPort)
	log.Println()

	// Create Raft node
	applyCh := make(chan raft.ApplyMsg, 100)
	node := raft.NewRaftNode(*nodeID, peerList, applyCh)

	// Build peer URLs (for HTTP transport)
	peerURLs := make(map[string]string)
	for _, peer := range peerList {
		// Peer URLs use their container names and Raft port
		peerURLs[peer] = fmt.Sprintf("http://%s:%s", peer, *raftPort)
	}

	// Create HTTP transport
	selfURL := fmt.Sprintf("http://%s:%s", *nodeID, *raftPort)
	transport := raft.NewTransport(node, selfURL, peerURLs)
	node.SetTransport(transport)

	// Start Raft HTTP server
	raftServer := raft.NewHTTPServer(node)
	go func() {
		log.Printf("🔗 Raft RPC server listening on :%s", *raftPort)
		if err := raftServer.StartServer(":" + *raftPort); err != nil {
			log.Fatalf("❌ Raft server failed: %v", err)
		}
	}()

	// Start Raft ledger (processes committed entries)
	raftLedger := ledger.NewRaftLedger(node, applyCh)
	raftLedger.Start()

	// Start REST API server
	apiServer := api.NewAPIServer(node, raftLedger, *nodeID, ":"+*apiPort)
	go func() {
		log.Printf("📡 REST API server listening on :%s", *apiPort)
		if err := apiServer.StartServer(); err != nil {
			log.Fatalf("❌ API server failed: %v", err)
		}
	}()

	// Start Raft consensus
	node.Start()

	log.Println("✅ RaftPay node started successfully!")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println()
	log.Println("🛑 Shutting down gracefully...")
	node.Stop()
	log.Println("✅ Shutdown complete")
}
