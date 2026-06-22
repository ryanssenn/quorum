import { NodeGrid } from "./nodes.js";
import { EventStream } from "./events.js";
import { Controls } from "./controls.js";

const grafanaBase = "http://localhost:3000";
const dashboardUID = "raft-playground";

function setupGrafanaPanels() {
  const params = "orgId=1&refresh=5s&kiosk";
  document.getElementById("panel-commit").src =
    `${grafanaBase}/d-solo/${dashboardUID}/raft?${params}&panelId=1`;
  document.getElementById("panel-lag").src =
    `${grafanaBase}/d-solo/${dashboardUID}/raft?${params}&panelId=2`;
  document.getElementById("panel-term").src =
    `${grafanaBase}/d-solo/${dashboardUID}/raft?${params}&panelId=3`;
}

const nodeGrid = new NodeGrid(document.getElementById("node-grid"));
const stream = new EventStream(nodeGrid, (state) => controls.onState(state));
const controls = new Controls(stream, nodeGrid);

stream.connect();
setupGrafanaPanels();

fetch("/api/cluster/status")
  .then((r) => r.json())
  .then((data) => {
    if (!data.clusterStarted && data.nodeCount) {
      document.getElementById("node-count").value = data.nodeCount;
      document.getElementById("node-count-val").textContent = data.nodeCount;
    }
  })
  .catch(() => {});
