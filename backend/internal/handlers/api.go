package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/pfrancistechs-cel/raftcore/backend/internal/cluster"
)

type API struct {
	Cluster *cluster.Cluster
}

func NewAPI(c *cluster.Cluster) *API {
	return &API{Cluster: c}
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "raftcore-consensus-engine",
		"version": "1.0.0",
		"moat":    "Raft consensus with Quorum Leader Election, AppendEntries replication & partition tolerance",
	})
}

func (a *API) GetClusterHandler(w http.ResponseWriter, r *http.Request) {
	state := a.Cluster.GetClusterState()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"nodes":      state,
		"node_count": len(state),
		"quorum":     (len(state)/2) + 1,
	})
}

func (a *API) ProposeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "command string required"})
		return
	}

	leaderID, idx, term, err := a.Cluster.ProposeCommand(req.Command)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "PROPOSED",
		"leader_id": leaderID,
		"index":     idx,
		"term":      term,
		"command":   req.Command,
	})
}

func (a *API) TogglePartitionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NodeID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id required"})
		return
	}

	newStatus := a.Cluster.TogglePartition(req.NodeID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":        req.NodeID,
		"is_partitioned": newStatus,
	})
}

func (a *API) TriggerElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NodeID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id required"})
		return
	}

	if err := a.Cluster.TriggerElection(req.NodeID); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ELECTION_INITIATED",
		"node_id": req.NodeID,
	})
}

func respondJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

