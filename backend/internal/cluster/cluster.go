package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pfrancistechs-cel/raftcore/backend/internal/raft"
)

type Cluster struct {
	mu         sync.RWMutex
	nodes      map[string]*raft.Node
	partitions map[string]bool
	stopCh     chan struct{}
}

func NewCluster(nodeCount int) *Cluster {
	if nodeCount < 3 {
		nodeCount = 5
	}

	c := &Cluster{
		nodes:      make(map[string]*raft.Node),
		partitions: make(map[string]bool),
		stopCh:     make(chan struct{}),
	}

	nodeIDs := make([]string, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeIDs[i] = fmt.Sprintf("node-%d", i+1)
	}

	for _, id := range nodeIDs {
		peers := make([]string, 0)
		for _, peer := range nodeIDs {
			if peer != id {
				peers = append(peers, peer)
			}
		}
		c.nodes[id] = raft.NewNode(id, peers, c)
	}

	c.startBackgroundLoops()
	return c
}

func (c *Cluster) SendRequestVote(target string, args raft.RequestVoteArgs) (raft.RequestVoteReply, error) {
	c.mu.RLock()
	node, ok := c.nodes[target]
	partitioned := c.partitions[target]
	c.mu.RUnlock()

	if !ok || partitioned {
		return raft.RequestVoteReply{}, errors.New("network timeout / partition drop")
	}
	return node.HandleRequestVote(args), nil
}

func (c *Cluster) SendAppendEntries(target string, args raft.AppendEntriesArgs) (raft.AppendEntriesReply, error) {
	c.mu.RLock()
	node, ok := c.nodes[target]
	partitioned := c.partitions[target]
	c.mu.RUnlock()

	if !ok || partitioned {
		return raft.AppendEntriesReply{}, errors.New("network timeout / partition drop")
	}
	return node.HandleAppendEntries(args), nil
}

func (c *Cluster) startBackgroundLoops() {
	// Elect an initial leader on node-1
	go func() {
		time.Sleep(300 * time.Millisecond)
		c.mu.RLock()
		n1 := c.nodes["node-1"]
		c.mu.RUnlock()
		if n1 != nil {
			n1.StartElection()
		}
	}()

	// Periodic leader heartbeats
	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.mu.RLock()
				for _, n := range c.nodes {
					status := n.Status()
					if status.Role == raft.RoleLeader && !status.IsPartitioned {
						n.BroadcastHeartbeat()
					}
				}
				c.mu.RUnlock()
			}
		}
	}()
}

func (c *Cluster) ProposeCommand(cmd string) (string, uint64, uint64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, n := range c.nodes {
		status := n.Status()
		if status.Role == raft.RoleLeader && !status.IsPartitioned {
			idx, term, ok := n.Propose(cmd)
			if ok {
				return status.ID, idx, term, nil
			}
		}
	}
	return "", 0, 0, errors.New("no active accessible leader available in current cluster partition")
}

func (c *Cluster) TogglePartition(nodeID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.nodes[nodeID]
	if !ok {
		return false
	}

	newVal := !c.partitions[nodeID]
	c.partitions[nodeID] = newVal
	node.SetPartitioned(newVal)

	// If former leader got partitioned, other nodes trigger election
	if newVal {
		go func() {
			time.Sleep(200 * time.Millisecond)
			c.mu.RLock()
			for id, n := range c.nodes {
				if id != nodeID && !c.partitions[id] {
					n.StartElection()
					break
				}
			}
			c.mu.RUnlock()
		}()
	}

	return newVal
}

func (c *Cluster) TriggerElection(nodeID string) error {
	c.mu.RLock()
	node, ok := c.nodes[nodeID]
	c.mu.RUnlock()

	if !ok {
		return errors.New("node not found")
	}
	go node.StartElection()
	return nil
}

func (c *Cluster) GetClusterState() []raft.NodeStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]raft.NodeStatus, 0, len(c.nodes))
	for i := 1; i <= len(c.nodes); i++ {
		id := fmt.Sprintf("node-%d", i)
		if n, ok := c.nodes[id]; ok {
			result = append(result, n.Status())
		}
	}
	return result
}


<!-- // Partition healing convergence latency reduced
 -->

<!-- // Network partition simulator -->
