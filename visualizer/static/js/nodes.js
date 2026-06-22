export class NodeGrid {
  constructor(container, onSelect) {
    this.container = container;
    this.onSelect = onSelect;
    this.cards = {};
    this.selectedId = null;
  }

  sync(nodes) {
    const ids = new Set(nodes.map((n) => n.id));
    for (const id of Object.keys(this.cards)) {
      if (!ids.has(id)) {
        this.cards[id].remove();
        delete this.cards[id];
      }
    }

    for (const node of nodes) {
      let card = this.cards[node.id];
      if (!card) {
        card = this._createCard(node.id);
        this.cards[node.id] = card;
        this.container.appendChild(card);
      }
      this._updateCard(card, node);
    }
  }

  setSelected(id) {
    this.selectedId = id;
    for (const [nodeId, card] of Object.entries(this.cards)) {
      card.classList.toggle("selected", nodeId === id);
    }
    return id;
  }

  getSelected() {
    return this.selectedId;
  }

  _createCard(id) {
    const card = document.createElement("article");
    card.className = "node-card";
    card.dataset.nodeId = id;
    card.innerHTML = `
      <header class="node-head">
        <span class="node-id"></span>
        <span class="node-role"></span>
      </header>
      <div class="node-meta"></div>
      <ol class="log-list"></ol>
    `;
    card.addEventListener("click", () => {
      this.setSelected(id);
      if (this.onSelect) this.onSelect(id);
    });
    return card;
  }

  _updateCard(card, node) {
    const role = node.running === false ? "offline" : (node.stateName || ["follower", "candidate", "leader"][node.state] || "follower");
    card.classList.toggle("offline", !node.running);
    card.classList.toggle("leader", role === "leader");
    card.classList.toggle("selected", node.id === this.selectedId);

    card.querySelector(".node-id").textContent = node.id;
    card.querySelector(".node-role").textContent = role;
    card.querySelector(".node-meta").textContent = node.running
      ? `term ${node.term ?? "-"} · commit ${node.commitIndex ?? "-"}`
      : "offline";

    const list = card.querySelector(".log-list");
    const entries = node.entries || [];
    list.innerHTML = entries.map((e) => this._entryRow(e, node.commitIndex)).join("");
  }

  _entryRow(entry, commitIndex) {
    const committed = entry.index <= commitIndex;
    const detail = entry.op === "put"
      ? `${entry.key}=${entry.value}`
      : entry.key;
    return `<li class="log-entry ${committed ? "committed" : "pending"}">
      <span class="log-index">#${entry.index}</span>
      <span class="log-op">${entry.op}</span>
      <span class="log-detail">${detail}</span>
      <span class="log-state">${committed ? "committed" : "pending"}</span>
    </li>`;
  }
}
