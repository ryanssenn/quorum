function quorumNeeded(nodeCount) {
  return Math.floor(nodeCount / 2) + 1;
}

export class EventStream {
  constructor(nodeGrid, onState) {
    this.nodeGrid = nodeGrid;
    this.onState = onState;
    this.source = null;
  }

  connect() {
    if (this.source) this.source.close();
    this.source = new EventSource("/api/stream");
    this.source.onmessage = (ev) => {
      try {
        this.handleSnapshot(JSON.parse(ev.data));
      } catch (_) { /* ignore */ }
    };
    this.source.onerror = () => setTimeout(() => this.connect(), 2000);
  }

  handleSnapshot(data) {
    const nodes = data.nodes || [];
    this.nodeGrid.sync(nodes);
    if (this.onState) {
      this.onState({
        nodes,
        clusterStarted: data.clusterStarted,
        partitionActive: data.partitionActive,
        partitionNodes: data.partitionNodes || [],
        log: data.log || [],
        scenario: data.scenario,
      });
    }
  }
}

export function updateHUD(state) {
  const nodes = state.nodes || [];
  const running = nodes.filter((n) => n.running);
  const leader = running.find((n) => n.state === 2 || n.stateName === "leader");
  const maxCommit = running.reduce((m, n) => Math.max(m, n.commitIndex ?? -1), -1);

  document.getElementById("hud-term").textContent = leader?.term ?? running[0]?.term ?? "-";
  document.getElementById("hud-commit").textContent = maxCommit >= 0 ? maxCommit : "-";
  document.getElementById("hud-leader").textContent = leader?.id ?? "none";

  const needed = quorumNeeded(nodes.length || 5);
  let acks = running.length;
  if (leader?.matchIndex) {
    acks = 1;
    for (const v of Object.values(leader.matchIndex)) {
      if (v >= maxCommit) acks++;
    }
  }
  document.getElementById("hud-quorum").textContent = `${Math.min(acks, needed)} / ${needed}`;
}
