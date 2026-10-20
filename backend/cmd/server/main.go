package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pfrancistechs-cel/raftcore/backend/internal/cluster"
	"github.com/pfrancistechs-cel/raftcore/backend/internal/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	cl := cluster.NewCluster(5)
	api := handlers.NewAPI(cl)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("/api/v1/cluster", api.GetClusterHandler)
	mux.HandleFunc("/api/v1/propose", api.ProposeHandler)
	mux.HandleFunc("/api/v1/partition", api.TogglePartitionHandler)
	mux.HandleFunc("/api/v1/elect", api.TriggerElectionHandler)

	handler := handlers.EnableCORS(mux)

	serverAddr := ":" + port
	fmt.Printf("🛡️ RaftCore 5-Node Consensus Cluster running on http://localhost%s\n", serverAddr)
	if err := http.ListenAndServe(serverAddr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

