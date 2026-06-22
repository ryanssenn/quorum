import { updateHUD } from "./events.js";

const API = {
  async post(path, body) {
    const res = await fetch(path, {
      method: "POST",
      headers: body ? { "Content-Type": "application/json" } : {},
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) throw new Error(await res.text() || res.statusText);
    return res.json().catch(() => ({}));
  },
};

export class Controls {
  constructor(stream, nodeGrid) {
    this.stream = stream;
    this.nodeGrid = nodeGrid;
    this.clusterStarted = false;
    this.bindElements();
  }

  bindElements() {
    this.els = {
      nodeCount: document.getElementById("node-count"),
      nodeCountVal: document.getElementById("node-count-val"),
      btnCreate: document.getElementById("btn-create"),
      btnStart: document.getElementById("btn-start"),
      btnStop: document.getElementById("btn-stop"),
      health: document.getElementById("cluster-health"),
      targetNode: document.getElementById("target-node"),
      reqKey: document.getElementById("req-key"),
      reqValue: document.getElementById("req-value"),
      btnPut: document.getElementById("btn-put"),
      btnGet: document.getElementById("btn-get"),
      selectedNode: document.getElementById("selected-node"),
      btnKill: document.getElementById("btn-kill"),
      btnRestart: document.getElementById("btn-restart"),
      btnPartition: document.getElementById("btn-partition"),
      btnClearPartition: document.getElementById("btn-clear-partition"),
      eventLog: document.getElementById("event-log"),
      tourSelect: document.getElementById("tour-select"),
      btnTourLoad: document.getElementById("btn-tour-load"),
      btnTourRun: document.getElementById("btn-tour-run"),
      btnTourPause: document.getElementById("btn-tour-pause"),
    };

    this.els.nodeCount.addEventListener("input", () => {
      this.els.nodeCountVal.textContent = this.els.nodeCount.value;
    });
    this.els.btnCreate.addEventListener("click", () => this.createCluster());
    this.els.btnStart.addEventListener("click", () => this.startCluster());
    this.els.btnStop.addEventListener("click", () => this.stopCluster());
    this.els.btnPut.addEventListener("click", () => this.sendRequest("put"));
    this.els.btnGet.addEventListener("click", () => this.sendRequest("get"));
    this.els.btnKill.addEventListener("click", () => this.killSelected());
    this.els.btnRestart.addEventListener("click", () => this.restartSelected());
    this.els.btnPartition.addEventListener("click", () => this.applyPartition());
    this.els.btnClearPartition.addEventListener("click", () => this.clearPartition());
    this.els.btnTourLoad.addEventListener("click", () => this.loadTour());
    this.els.btnTourRun.addEventListener("click", () => this.runTour());
    this.els.btnTourPause.addEventListener("click", () => this.pauseTour());

    this.nodeGrid.onSelect = (id) => {
      this.els.selectedNode.textContent = id;
      this.updateActionButtons();
    };
  }

  onState(state) {
    this.clusterStarted = !!state.clusterStarted;
    updateHUD(state);
    this.updateHealth(state);
    this.updateTargetSelect(state.nodes || []);
    this.updateLog(state.log || []);
    this.updateActionButtons(state);
  }

  updateHealth(state) {
    const nodes = state.nodes || [];
    const running = nodes.filter((n) => n.running).length;
    const el = this.els.health;
    if (!state.clusterStarted) {
      el.textContent = "Not started";
      el.className = "health-chip";
      return;
    }
    const leader = nodes.some((n) => n.running && (n.state === 2 || n.stateName === "leader"));
    el.textContent = leader ? `${running}/${nodes.length} nodes, leader ok` : `${running}/${nodes.length} nodes, electing`;
    el.className = "health-chip " + (leader ? "healthy" : "warning");
  }

  updateTargetSelect(nodes) {
    const prev = this.els.targetNode.value;
    this.els.targetNode.innerHTML = "";
    for (const node of nodes) {
      if (!node.running) continue;
      const opt = document.createElement("option");
      opt.value = node.id;
      opt.textContent = node.id;
      this.els.targetNode.appendChild(opt);
    }
    if (prev && [...this.els.targetNode.options].some((o) => o.value === prev)) {
      this.els.targetNode.value = prev;
    }
  }

  updateLog(lines) {
    if (!this.els.eventLog) return;
    this.els.eventLog.innerHTML = lines.slice(-40).map((l) => `<div class="line">${escapeHtml(l)}</div>`).join("");
    this.els.eventLog.scrollTop = this.els.eventLog.scrollHeight;
  }

  updateActionButtons(state = {}) {
    const started = !!state.clusterStarted;
    const selected = this.nodeGrid.getSelected();
    this.els.btnStart.disabled = started;
    this.els.btnStop.disabled = !started;
    this.els.btnPut.disabled = !started;
    this.els.btnGet.disabled = !started;
    this.els.btnKill.disabled = !selected;
    this.els.btnRestart.disabled = !selected;
    this.els.btnPartition.disabled = !selected;
    this.els.btnClearPartition.disabled = !state.partitionActive;
    this.els.btnTourRun.disabled = !started;
  }

  async createCluster() {
    try {
      await API.post("/api/cluster/create", { nodes: parseInt(this.els.nodeCount.value, 10) });
    } catch (e) { alert(e.message); }
  }

  async startCluster() {
    try { await API.post("/api/cluster/start"); } catch (e) { alert(e.message); }
  }

  async stopCluster() {
    try {
      await API.post("/api/cluster/stop");
      this.nodeGrid.setSelected(null);
      this.els.selectedNode.textContent = "None selected";
    } catch (e) { alert(e.message); }
  }

  async sendRequest(op) {
    const node = this.els.targetNode.value;
    const key = this.els.reqKey.value.trim();
    if (!node || !key) return;
    try {
      const res = await API.post("/api/request", {
        client: "client-A",
        op,
        key,
        value: this.els.reqValue.value,
        node,
      });
      if (op === "get" && res.result) this.els.reqValue.value = res.result;
    } catch (e) { alert(e.message); }
  }

  async killSelected() {
    const id = this.nodeGrid.getSelected();
    if (!id) return;
    try { await API.post(`/api/cluster/nodes/${id}/kill`); } catch (e) { alert(e.message); }
  }

  async restartSelected() {
    const id = this.nodeGrid.getSelected();
    if (!id) return;
    try { await API.post(`/api/cluster/nodes/${id}/restart`); } catch (e) { alert(e.message); }
  }

  async applyPartition() {
    const id = this.nodeGrid.getSelected();
    if (!id) return;
    try { await API.post("/api/cluster/partition", { isolated: [id] }); } catch (e) { alert(e.message); }
  }

  async clearPartition() {
    try { await API.post("/api/cluster/partition/clear"); } catch (e) { alert(e.message); }
  }

  async loadTour() {
    const path = this.els.tourSelect.value;
    if (!path) return;
    try { await API.post("/api/scenario/load", { path }); } catch (e) { alert(e.message); }
  }

  async runTour() {
    try { await API.post("/api/scenario/run"); } catch (e) { alert(e.message); }
  }

  async pauseTour() {
    try { await API.post("/api/scenario/pause"); } catch (e) { alert(e.message); }
  }
}

function escapeHtml(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
