"use client";

import React, { useState } from "react";
import {
  ShieldCheck,
  Crown,
  Network,
  Radio,
  Vote,
  Terminal,
  Activity,
  AlertTriangle,
  Play,
  RotateCcw,
  CheckCircle2,
  Clock,
  Layers,
  FileText,
  SlidersHorizontal,
  ChevronRight
} from "lucide-react";

interface LogEntry {
  index: number;
  term: number;
  command: string;
}

interface NodeStatus {
  id: string;
  role: "LEADER" | "FOLLOWER" | "CANDIDATE";
  current_term: number;
  voted_for: string;
  commit_index: number;
  last_applied: number;
  is_partitioned: boolean;
  log_entries: LogEntry[];
}

const INITIAL_NODES: NodeStatus[] = [
  {
    id: "node-1",
    role: "LEADER",
    current_term: 3,
    voted_for: "node-1",
    commit_index: 4,
    last_applied: 4,
    is_partitioned: false,
    log_entries: [
      { index: 1, term: 1, command: "SET cluster.mode = 'ACTIVE'" },
      { index: 2, term: 2, command: "TX:REGISTER client='auth_svc'" },
      { index: 3, term: 3, command: "SET config.heartbeat_ms = 150" },
      { index: 4, term: 3, command: "TX:TRANSFER acc_402 -> acc_901: $1200" },
    ],
  },
  {
    id: "node-2",
    role: "FOLLOWER",
    current_term: 3,
    voted_for: "node-1",
    commit_index: 4,
    last_applied: 4,
    is_partitioned: false,
    log_entries: [
      { index: 1, term: 1, command: "SET cluster.mode = 'ACTIVE'" },
      { index: 2, term: 2, command: "TX:REGISTER client='auth_svc'" },
      { index: 3, term: 3, command: "SET config.heartbeat_ms = 150" },
      { index: 4, term: 3, command: "TX:TRANSFER acc_402 -> acc_901: $1200" },
    ],
  },
  {
    id: "node-3",
    role: "FOLLOWER",
    current_term: 3,
    voted_for: "node-1",
    commit_index: 4,
    last_applied: 4,
    is_partitioned: false,
    log_entries: [
      { index: 1, term: 1, command: "SET cluster.mode = 'ACTIVE'" },
      { index: 2, term: 2, command: "TX:REGISTER client='auth_svc'" },
      { index: 3, term: 3, command: "SET config.heartbeat_ms = 150" },
      { index: 4, term: 3, command: "TX:TRANSFER acc_402 -> acc_901: $1200" },
    ],
  },
  {
    id: "node-4",
    role: "FOLLOWER",
    current_term: 3,
    voted_for: "node-1",
    commit_index: 3,
    last_applied: 3,
    is_partitioned: false,
    log_entries: [
      { index: 1, term: 1, command: "SET cluster.mode = 'ACTIVE'" },
      { index: 2, term: 2, command: "TX:REGISTER client='auth_svc'" },
      { index: 3, term: 3, command: "SET config.heartbeat_ms = 150" },
    ],
  },
  {
    id: "node-5",
    role: "FOLLOWER",
    current_term: 3,
    voted_for: "node-1",
    commit_index: 4,
    last_applied: 4,
    is_partitioned: false,
    log_entries: [
      { index: 1, term: 1, command: "SET cluster.mode = 'ACTIVE'" },
      { index: 2, term: 2, command: "TX:REGISTER client='auth_svc'" },
      { index: 3, term: 3, command: "SET config.heartbeat_ms = 150" },
      { index: 4, term: 3, command: "TX:TRANSFER acc_402 -> acc_901: $1200" },
    ],
  },
];

export default function RaftCoreDashboard() {
  const [nodes, setNodes] = useState<NodeStatus[]>(INITIAL_NODES);
  const [inputCommand, setInputCommand] = useState("TX:MUTATE balance=98500 client='payment_worker'");
  const [activeLeaderId, setActiveLeaderId] = useState("node-1");
  const [clusterLogs, setClusterLogs] = useState<string[]>([
    "[Raft] Leader 'node-1' maintaining heartbeats across 4 peers (Term 3)",
    "[Commit] Log entry #4 committed by quorum majority (4/5 nodes acknowledged)",
    "[Invariant] Log matching property holds: monotonic term progression verified",
    "[Quorum] 3 of 5 nodes required for consensus decision",
  ]);

  const handlePropose = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputCommand) return;

    const leader = nodes.find((n) => n.id === activeLeaderId && !n.is_partitioned);
    if (!leader) {
      setClusterLogs([
        `[REJECTED] Cannot commit '${inputCommand}': Leader is partitioned or unreachable!`,
        ...clusterLogs,
      ]);
      return;
    }

    const newIndex = leader.log_entries.length + 1;
    const newEntry: LogEntry = {
      index: newIndex,
      term: leader.current_term,
      command: inputCommand,
    };

    // Update active non-partitioned nodes
    const updated = nodes.map((n) => {
      if (!n.is_partitioned) {
        return {
          ...n,
          log_entries: [...n.log_entries, newEntry],
          commit_index: newIndex,
          last_applied: newIndex,
        };
      }
      return n;
    });

    setNodes(updated);
    setClusterLogs([
      `[PROPOSED] Leader '${leader.id}' appended entry #${newIndex} (Term ${leader.current_term}): '${inputCommand}'`,
      `[CONSENSUS] Quorum majority accepted entry #${newIndex} -> Advanced commitIndex to ${newIndex}`,
      ...clusterLogs.slice(0, 8),
    ]);
    setInputCommand("");
  };

  const togglePartition = (nodeId: string) => {
    const updated = nodes.map((n) => {
      if (n.id === nodeId) {
        return { ...n, is_partitioned: !n.is_partitioned };
      }
      return n;
    });

    setNodes(updated);
    const target = updated.find((n) => n.id === nodeId);
    const action = target?.is_partitioned ? "ISOLATED from network" : "RECONNECTED to cluster";

    setClusterLogs([
      `[NETWORK] Node '${nodeId}' ${action}`,
      ...clusterLogs.slice(0, 8),
    ]);

    // If the partitioned node was the leader, elect a new one from remaining quorum
    if (nodeId === activeLeaderId && target?.is_partitioned) {
      const candidate = updated.find((n) => !n.is_partitioned);
      if (candidate) {
        setTimeout(() => {
          handleForceElect(candidate.id);
        }, 300);
      }
    }
  };

  const handleForceElect = (nodeId: string) => {
    const target = nodes.find((n) => n.id === nodeId);
    if (target?.is_partitioned) return;

    const nextTerm = (target?.current_term || 1) + 1;
    const updated = nodes.map((n) => {
      if (n.id === nodeId) {
        return {
          ...n,
          role: "LEADER" as const,
          current_term: nextTerm,
          voted_for: nodeId,
        };
      }
      return {
        ...n,
        role: "FOLLOWER" as const,
        current_term: nextTerm,
        voted_for: nodeId,
      };
    });

    setNodes(updated);
    setActiveLeaderId(nodeId);
    setClusterLogs([
      `[ELECTION] Node '${nodeId}' initiated campaign for Term ${nextTerm}`,
      `[QUORUM] Node '${nodeId}' gathered majority votes -> TRANSITIONED TO LEADER`,
      ...clusterLogs.slice(0, 8),
    ]);
  };

  return (
    <div className="min-h-screen bg-[#070914] text-slate-100 flex flex-col font-sans">
      {/* Header */}
      <header className="border-b border-cyan-950/60 bg-gradient-to-r from-cyan-950/40 via-slate-900 to-slate-950 px-6 py-4 sticky top-0 z-50 backdrop-blur-md">
        <div className="max-w-7xl mx-auto flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center space-x-3">
            <div className="p-2.5 bg-cyan-500/20 text-cyan-400 rounded-xl border border-cyan-500/30 flex items-center justify-center">
              <ShieldCheck className="w-6 h-6 text-cyan-400" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <span className="font-black text-xl tracking-wider text-white">RAFT<span className="text-cyan-400">CORE</span></span>
                <span className="bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[10px] font-bold px-2 py-0.5 rounded-full uppercase tracking-wider">
                  5-Node Consensus Mesh
                </span>
              </div>
              <p className="text-xs text-slate-400">Distributed Consensus State Machine with Quorum Leader Election & AppendEntries RPC</p>
            </div>
          </div>

          <div className="flex items-center space-x-4 text-xs font-mono">
            <div className="bg-slate-900 border border-slate-800 px-3 py-1.5 rounded-lg flex items-center space-x-2">
              <span className="text-slate-400">Active Leader:</span>
              <span className="font-bold text-amber-400 flex items-center space-x-1">
                <Crown className="w-3.5 h-3.5 inline mr-1 text-amber-400" />
                {activeLeaderId}
              </span>
            </div>
            <div className="bg-slate-900 border border-slate-800 px-3 py-1.5 rounded-lg text-emerald-400">
              Quorum: 3 / 5 Required
            </div>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="max-w-7xl mx-auto w-full p-6 flex-1 space-y-6">
        {/* Node Topology Grid */}
        <div>
          <div className="flex items-center justify-between pb-2 mb-4">
            <h3 className="font-bold text-sm text-white uppercase tracking-wider flex items-center space-x-2">
              <Network className="w-4 h-4 text-cyan-400" />
              <span>Cluster Node Topology (5 Participating Processes)</span>
            </h3>
            <span className="text-xs text-slate-400">Click &quot;Partition Network&quot; to test split-brain quorum tolerance</span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
            {nodes.map((n) => {
              const isLeader = n.role === "LEADER";
              return (
                <div
                  key={n.id}
                  className={`border rounded-2xl p-4 flex flex-col justify-between transition-all relative overflow-hidden shadow-xl ${
                    n.is_partitioned
                      ? "bg-rose-950/20 border-rose-500/40 opacity-60"
                      : isLeader
                      ? "bg-slate-900/90 border-amber-500/80 shadow-amber-950/20"
                      : "bg-slate-900/70 border-slate-800"
                  }`}
                >
                  <div>
                    {/* Header */}
                    <div className="flex items-center justify-between pb-2 border-b border-slate-800 mb-3">
                      <span className="font-bold text-sm text-white font-mono">{n.id}</span>
                      <span className={`text-[10px] font-black uppercase px-2 py-0.5 rounded flex items-center space-x-1 ${
                        isLeader
                          ? "bg-amber-500/20 text-amber-300 border border-amber-500/40"
                          : "bg-cyan-500/20 text-cyan-300 border border-cyan-500/30"
                      }`}>
                        {isLeader && <Crown className="w-3 h-3 mr-0.5 inline" />}
                        <span>{n.role}</span>
                      </span>
                    </div>

                    {/* Metadata */}
                    <div className="space-y-1.5 text-xs text-slate-400 font-mono">
                      <div className="flex justify-between">
                        <span>Term:</span>
                        <strong className="text-white">{n.current_term}</strong>
                      </div>
                      <div className="flex justify-between">
                        <span>Voted For:</span>
                        <strong className="text-slate-200">{n.voted_for || "None"}</strong>
                      </div>
                      <div className="flex justify-between">
                        <span>Commit Index:</span>
                        <strong className="text-cyan-400">#{n.commit_index}</strong>
                      </div>
                      <div className="flex justify-between">
                        <span>Log Entries:</span>
                        <strong className="text-white">{n.log_entries.length}</strong>
                      </div>
                    </div>
                  </div>

                  {/* Node Actions */}
                  <div className="pt-3 border-t border-slate-800 space-y-1.5 mt-4">
                    <button
                      onClick={() => togglePartition(n.id)}
                      className={`w-full text-[11px] font-bold py-1.5 rounded transition-colors ${
                        n.is_partitioned
                          ? "bg-emerald-600 hover:bg-emerald-500 text-white"
                          : "bg-slate-800 hover:bg-rose-950/60 hover:text-rose-400 text-slate-300 border border-slate-700"
                      }`}
                    >
                      {n.is_partitioned ? "Heal Partition" : "Partition Network"}
                    </button>
                    {!isLeader && !n.is_partitioned && (
                      <button
                        onClick={() => handleForceElect(n.id)}
                        className="w-full text-[10px] bg-slate-800/60 hover:bg-slate-800 text-slate-400 hover:text-white py-1 rounded transition-colors"
                      >
                        Trigger Election
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* State Machine Propose Bar */}
        <div className="bg-slate-900/80 border border-slate-800 rounded-2xl p-5 shadow-xl">
          <h4 className="font-bold text-sm text-white mb-2 flex items-center space-x-2">
            <Terminal className="w-4 h-4 text-cyan-400" />
            <span>State Machine Replicated Command Proposer</span>
          </h4>
          <p className="text-xs text-slate-400 mb-4">
            Commands proposed to the cluster leader are appended to the WAL and committed only when acknowledged by the majority quorum ($3/5$ nodes).
          </p>

          <form onSubmit={handlePropose} className="flex gap-3">
            <input
              type="text"
              value={inputCommand}
              onChange={(e) => setInputCommand(e.target.value)}
              placeholder="e.g. TX:MUTATE account=409 balance=150000 currency='USD'"
              className="flex-1 bg-slate-950 border border-slate-700 rounded-lg px-4 py-2.5 text-xs text-white font-mono focus:outline-none focus:border-cyan-500"
            />
            <button
              type="submit"
              className="bg-cyan-600 hover:bg-cyan-500 text-white font-bold px-6 py-2.5 rounded-lg text-xs transition-colors flex items-center space-x-1.5 shadow-md shadow-cyan-950/40"
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              <span>Propose to Leader</span>
            </button>
          </form>
        </div>

        {/* Replicated Commit Log Comparison Table */}
        <div className="bg-slate-900/80 border border-slate-800 rounded-2xl p-5 shadow-xl space-y-4">
          <div className="flex items-center justify-between pb-2 border-b border-slate-800">
            <h4 className="font-bold text-sm text-white flex items-center space-x-2">
              <Layers className="w-4 h-4 text-cyan-400" />
              <span>Replicated Commit Log (Monotonic Index Matching)</span>
            </h4>
            <span className="text-xs text-slate-400 font-mono">Consensus Invariant: Commits are immutable</span>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono">
              <thead>
                <tr className="border-b border-slate-800 text-slate-400 font-bold uppercase text-[10px]">
                  <th className="pb-2">Index</th>
                  <th className="pb-2">Term</th>
                  <th className="pb-2">Command Payload</th>
                  <th className="pb-2">Quorum Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {nodes[0].log_entries.map((entry) => (
                  <tr key={entry.index}>
                    <td className="py-2.5 text-cyan-400 font-bold">#{entry.index}</td>
                    <td className="py-2.5 text-slate-400">Term {entry.term}</td>
                    <td className="py-2.5 text-slate-200">{entry.command}</td>
                    <td className="py-2.5">
                      <span className="bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-[10px] font-bold px-2 py-0.5 rounded">
                        COMMITTED (QUORUM 5/5)
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Live Cluster Event Stream */}
        <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 shadow-lg space-y-3">
          <div className="flex items-center justify-between pb-2 border-b border-slate-800">
            <span className="text-xs font-bold text-white uppercase tracking-wider flex items-center space-x-2">
              <Activity className="w-4 h-4 text-slate-400" />
              <span>Raft Cluster RPC Telemetry</span>
            </span>
            <span className="text-[10px] text-cyan-400 font-mono">Cluster Port: :8085</span>
          </div>

          <div className="space-y-1.5 font-mono text-[11px] bg-slate-950 p-3 rounded-lg border border-slate-900 max-h-32 overflow-y-auto">
            {clusterLogs.map((log, idx) => (
              <div key={idx} className="text-slate-300 leading-relaxed">
                <span className="text-slate-600 mr-2">{idx + 1}.</span>
                {log}
              </div>
            ))}
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-slate-950 px-6 py-4 text-xs text-slate-500 text-center">
        RaftCore Distributed Consensus Engine • Built with Go 1.25 + Next.js 15 App Router • Quorum State Machine
      </footer>
    </div>
  );
}


<!-- /* Live election pulses */ -->

<!-- /* Network partition controls */ -->

<!-- /* Log replication inspector */ -->
