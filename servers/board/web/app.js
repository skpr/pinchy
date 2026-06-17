// Pinchy Board frontend. Polls the board API and renders a kanban of opencode
// sessions grouped into per-project swimlanes, with interactive card actions.
// A second tab lists all pinchy Environments with a delete action.

const POLL_INTERVAL_MS = 3000;

const COLUMN_LABELS = {
  working: "Working",
  "needs-input": "Needs input",
  idle: "Idle",
  error: "Error",
};

// phase → CSS class (reuses the board status palette)
const PHASE_CLASS = {
  Running: "idle",
  Pending: "working",
  Failed: "error",
};

let opencodeWebURL = "";
let pollTimer = null;
let activeTab = "board"; // "board" | "environments"

const boardEl = document.getElementById("page-board");
const envsEl = document.getElementById("page-environments");
const statusDot = document.getElementById("status");
const statusText = document.getElementById("status-text");

// ── Utilities ────────────────────────────────────────────────────────────────

// escapeHTML guards against rendering arbitrary strings as markup.
function escapeHTML(value) {
  const div = document.createElement("div");
  div.textContent = value == null ? "" : String(value);
  return div.innerHTML;
}

// relativeTime renders an epoch-millis (or epoch-seconds) timestamp as "ago".
function relativeTime(ms) {
  if (!ms) return "";
  // createdAt from gRPC is Unix seconds; board card updated is millis.
  // Values smaller than 1e10 are treated as seconds.
  const epochMs = ms < 1e10 ? ms * 1000 : ms;
  const seconds = Math.max(0, Math.floor((Date.now() - epochMs) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function setStatus(ok, text) {
  statusDot.className = "status-dot " + (ok ? "ok" : "err");
  statusText.textContent = text;
}

// ── Config ───────────────────────────────────────────────────────────────────

async function loadConfig() {
  try {
    const res = await fetch("/api/config");
    const cfg = await res.json();
    opencodeWebURL = cfg.opencodeWebURL || "";
  } catch (_) {
    opencodeWebURL = "";
  }
}

// ── Tab switching ─────────────────────────────────────────────────────────────

document.querySelectorAll(".tab").forEach((btn) => {
  btn.addEventListener("click", () => {
    activeTab = btn.dataset.tab;
    document.querySelectorAll(".tab").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");

    boardEl.classList.toggle("hidden", activeTab !== "board");
    envsEl.classList.toggle("hidden", activeTab !== "environments");

    // Restart the poll loop for the newly active page immediately.
    if (pollTimer) clearTimeout(pollTimer);
    poll();
  });
});

// ── Poll loop ─────────────────────────────────────────────────────────────────

async function poll() {
  try {
    if (activeTab === "board") {
      const res = await fetch("/api/board");
      if (!res.ok) throw new Error(`status ${res.status}`);
      renderBoard(await res.json());
    } else {
      const res = await fetch("/api/environments");
      if (!res.ok) throw new Error(`status ${res.status}`);
      renderEnvironments(await res.json());
    }
    setStatus(true, `updated ${new Date().toLocaleTimeString()}`);
  } catch (_) {
    setStatus(false, activeTab === "board" ? "opencode unreachable" : "pinchy-api unreachable");
  } finally {
    pollTimer = setTimeout(poll, POLL_INTERVAL_MS);
  }
}

// action performs an API mutation then immediately re-polls.
async function action(method, path, body) {
  try {
    const opts = { method };
    if (body) {
      opts.headers = { "Content-Type": "application/json" };
      opts.body = JSON.stringify(body);
    }
    const res = await fetch(path, opts);
    if (!res.ok) throw new Error(`status ${res.status}`);
  } catch (_) {
    setStatus(false, "action failed");
  } finally {
    if (pollTimer) clearTimeout(pollTimer);
    poll();
  }
}

// ── Board page ────────────────────────────────────────────────────────────────

function cardActions(card) {
  const buttons = [];

  if (opencodeWebURL) {
    buttons.push(
      `<a href="${escapeHTML(opencodeWebURL)}" target="_blank" rel="noopener">OpenCode</a>`
    );
  }

  for (const env of card.envs || []) {
    buttons.push(
      `<a class="env-link" href="${escapeHTML(env.url)}" target="_blank" rel="noopener">:${escapeHTML(
        env.port
      )}</a>`
    );
  }

  if (card.status === "needs-input" && card.permissionID) {
    buttons.push(
      `<button class="approve" data-act="approve" data-id="${escapeHTML(
        card.id
      )}" data-perm="${escapeHTML(card.permissionID)}">Approve</button>`,
      `<button class="deny" data-act="deny" data-id="${escapeHTML(
        card.id
      )}" data-perm="${escapeHTML(card.permissionID)}">Deny</button>`
    );
  }

  if (card.status === "working") {
    buttons.push(
      `<button class="abort" data-act="abort" data-id="${escapeHTML(
        card.id
      )}">Abort</button>`
    );
  }

  buttons.push(
    `<button class="delete" data-act="delete-session" data-id="${escapeHTML(
      card.id
    )}">Delete</button>`
  );

  return `<div class="card-actions">${buttons.join("")}</div>`;
}

function renderCard(card) {
  const title = escapeHTML(card.title || "(untitled session)");
  const updated = relativeTime(card.updated);

  let diff = "";
  if (card.additions || card.deletions) {
    diff = `<span class="diff"><span class="add">+${card.additions || 0}</span> <span class="del">-${
      card.deletions || 0
    }</span></span>`;
  }

  return `
    <div class="card ${card.status}">
      <div class="card-title">${title}</div>
      <div class="card-meta">
        <span>${updated}</span>
        ${diff}
      </div>
      ${cardActions(card)}
    </div>`;
}

function renderBoard(board) {
  const columns = board.columns || [];
  const swimlanes = board.swimlanes || [];

  if (swimlanes.length === 0) {
    boardEl.innerHTML = `<p class="empty">No sessions yet. Start one in opencode.</p>`;
    return;
  }

  const html = swimlanes
    .map((lane) => {
      const byStatus = {};
      for (const col of columns) byStatus[col] = [];
      for (const card of lane.cards) {
        (byStatus[card.status] || (byStatus[card.status] = [])).push(card);
      }

      const cols = columns
        .map((col) => {
          const cards = byStatus[col] || [];
          const body =
            cards.length > 0
              ? cards.map(renderCard).join("")
              : `<div class="card-empty">—</div>`;
          return `
            <div class="column ${col}">
              <div class="column-title">
                <span class="col-pip"></span>${COLUMN_LABELS[col] || col}
                <span class="count">${cards.length}</span>
              </div>
              ${body}
            </div>`;
        })
        .join("");

      const directory = lane.directory || "(unknown)";

      return `
        <section class="swimlane">
          <div class="swimlane-header">
            <span class="worktree">${escapeHTML(directory)}</span>
          </div>
          <div class="columns">${cols}</div>
        </section>`;
    })
    .join("");

  boardEl.innerHTML = html;
}

// Delegate board card action clicks.
boardEl.addEventListener("click", (event) => {
  const btn = event.target.closest("button[data-act]");
  if (!btn) return;

  const id = btn.dataset.id;
  const act = btn.dataset.act;

  switch (act) {
    case "approve":
      action("POST", `/api/sessions/${encodeURIComponent(id)}/permission`, {
        permissionID: btn.dataset.perm,
        response: "always",
      });
      break;
    case "deny":
      action("POST", `/api/sessions/${encodeURIComponent(id)}/permission`, {
        permissionID: btn.dataset.perm,
        response: "reject",
      });
      break;
    case "abort":
      action("POST", `/api/sessions/${encodeURIComponent(id)}/abort`);
      break;
    case "delete-session":
      if (confirm("Delete this session and all its data?")) {
        action("DELETE", `/api/sessions/${encodeURIComponent(id)}`);
      }
      break;
  }
});

// ── Environments page ─────────────────────────────────────────────────────────

function renderEnvironments(envs) {
  if (!Array.isArray(envs) || envs.length === 0) {
    envsEl.innerHTML = `<p class="empty">No environments running.</p>`;
    return;
  }

  const rows = envs
    .map((env) => {
      const phaseClass = PHASE_CLASS[env.phase] || "idle";
      const sessionLink =
        opencodeWebURL && env.sessionID
          ? `<a href="${escapeHTML(opencodeWebURL)}" target="_blank" rel="noopener">${escapeHTML(
              env.sessionID
            )}</a>`
          : escapeHTML(env.sessionID || "—");

      return `
        <tr>
          <td class="mono">${escapeHTML(env.name)}</td>
          <td class="mono">${sessionLink}</td>
          <td class="mono">${escapeHTML(env.path || "—")}</td>
          <td><span class="phase-badge ${phaseClass}">${escapeHTML(env.phase || "—")}</span></td>
          <td class="mono">${escapeHTML(env.podIP || "—")}</td>
          <td>${relativeTime(env.createdAt)}</td>
          <td>
            <button class="delete" data-act="delete-env" data-name="${escapeHTML(env.name)}">Delete</button>
          </td>
        </tr>`;
    })
    .join("");

  envsEl.innerHTML = `
    <table class="env-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Session</th>
          <th>Path</th>
          <th>Phase</th>
          <th>Pod IP</th>
          <th>Age</th>
          <th></th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>`;
}

// Delegate environments page action clicks.
envsEl.addEventListener("click", (event) => {
  const btn = event.target.closest("button[data-act]");
  if (!btn) return;

  if (btn.dataset.act === "delete-env") {
    action("DELETE", `/api/environments/${encodeURIComponent(btn.dataset.name)}`);
  }
});

// ── Boot ──────────────────────────────────────────────────────────────────────

(async function start() {
  await loadConfig();
  poll();
})();
