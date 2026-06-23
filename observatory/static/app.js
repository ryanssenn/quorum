const grafanaBase = "http://localhost:3000";
const dashboardPath = "/d/raft-observatory/raft-observatory?orgId=1&refresh=5s&kiosk";

async function apiPost(path, body) {
  const res = await fetch(path, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(await res.text() || res.statusText);
  return res.json().catch(() => ({}));
}

function setupDashboard() {
  document.getElementById("grafana-dashboard").src = grafanaBase + dashboardPath;
}

function updateStatus(data) {
  const nodes = data.nodes || [];
  const running = nodes.filter((n) => n.running);
  const leader = running.find((n) => n.state === 2 || n.stateName === "leader");
  const maxCommit = running.reduce((m, n) => Math.max(m, n.commitIndex ?? -1), -1);

  document.getElementById("stat-leader").textContent = leader?.id ?? "none";
  document.getElementById("stat-term").textContent = leader?.term ?? running[0]?.term ?? "-";
  document.getElementById("stat-commit").textContent = maxCommit >= 0 ? maxCommit : "-";
  document.getElementById("stat-running").textContent = `${running.length}/${nodes.length}`;

  const health = document.getElementById("cluster-health");
  if (!data.clusterStarted) {
    health.textContent = "Not started";
    health.className = "health";
  } else {
    health.textContent = leader ? `${running.length} nodes, leader ok` : `${running.length} nodes, electing`;
    health.className = "health " + (leader ? "ok" : "");
  }

  document.getElementById("btn-start").disabled = data.clusterStarted;
  document.getElementById("btn-stop").disabled = !data.clusterStarted;
  document.getElementById("btn-run").disabled = !data.clusterStarted;
}

function updateScenario(sc) {
  const el = document.getElementById("scenario-status");
  if (!sc.loaded) {
    el.textContent = "No scenario";
    return;
  }
  el.textContent = sc.running
    ? `Running: ${sc.currentStep || sc.name} (${sc.stepIndex + 1}/${sc.totalSteps})`
    : sc.done ? `Done: ${sc.name}` : `Loaded: ${sc.name}`;
  document.getElementById("btn-pause").disabled = !sc.running;

  const log = document.getElementById("event-log");
  log.innerHTML = (sc.log || []).slice(-30).map((l) => `<div>${escapeHtml(l)}</div>`).join("");
  log.scrollTop = log.scrollHeight;
}

function escapeHtml(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

async function poll() {
  try {
    const [cluster, scenario] = await Promise.all([
      fetch("/api/cluster/status").then((r) => r.json()),
      fetch("/api/scenario").then((r) => r.json()),
    ]);
    updateStatus(cluster);
    updateScenario(scenario);
  } catch (_) { /* ignore */ }
}

document.getElementById("node-count").addEventListener("input", (e) => {
  document.getElementById("node-count-val").textContent = e.target.value;
});

document.getElementById("btn-create").addEventListener("click", async () => {
  try {
    await apiPost("/api/cluster/create", { nodes: parseInt(document.getElementById("node-count").value, 10) });
  } catch (e) { alert(e.message); }
});

document.getElementById("btn-start").addEventListener("click", async () => {
  try { await apiPost("/api/cluster/start"); } catch (e) { alert(e.message); }
});

document.getElementById("btn-stop").addEventListener("click", async () => {
  try { await apiPost("/api/cluster/stop"); } catch (e) { alert(e.message); }
});

document.getElementById("btn-load").addEventListener("click", async () => {
  const path = document.getElementById("scenario-select").value;
  if (!path) return;
  try { await apiPost("/api/scenario/load", { path }); } catch (e) { alert(e.message); }
});

document.getElementById("btn-run").addEventListener("click", async () => {
  try { await apiPost("/api/scenario/run"); } catch (e) { alert(e.message); }
});

document.getElementById("btn-pause").addEventListener("click", async () => {
  try { await apiPost("/api/scenario/pause"); } catch (e) { alert(e.message); }
});

setupDashboard();
poll();
setInterval(poll, 1000);
