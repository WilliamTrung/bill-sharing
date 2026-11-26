// Realtime keeps one WebSocket open per tab and turns server events into
// notifications plus page refreshes. It is intentionally small: the socket only
// signals *what* changed, and pages refetch authoritative data themselves.
const Realtime = (() => {
  const handlers = new Map(); // event type -> [fn]
  const anyHandlers = []; // fn(event) for every event
  const entries = []; // notification inbox, newest first
  const MAX_ENTRIES = 50;
  let socket = null;
  let attempt = 0;
  let reconnectTimer = null;
  let manualClose = false;

  // --- notifications from events -------------------------------------------

  // describe turns an event into bell text. Returns null for silent updates.
  function describe(event) {
    if (event.type === "group.created") {
      return {
        text: I18n.t("notif.groupCreated", { group: event.group.name }),
        href: "/admin",
      };
    }
    if (event.type === "group.updated") {
      return {
        text: I18n.t("notif.groupUpdated", { group: event.group.name }),
        href: "/admin",
      };
    }
    if (!event.bill) return null;

    const title = event.bill.title || "";
    const projectId = Chrome.currentProjectID ? Chrome.currentProjectID() : 0;
    const href = event.bill.bill_id
      ? `/p/${projectId}/bill-detail.html?id=${event.bill.bill_id}`
      : `/p/${projectId}/dashboard.html`;
    const amount = Money.format(event.bill.total_amount_cents || 0, { signed: false });
    const actor = event.bill.actor_name || "";

    switch (event.type) {
      case "bill.created":
        return {
          text: event.bill.actor_name
            ? I18n.t("notif.billCreatedBy", { actor, title, amount })
            : I18n.t("notif.billCreated", { title, amount }),
          href,
        };
      case "bill.settled":
        return { text: I18n.t("notif.billSettled", { title }), href };
      case "bill.settle_requested":
        return { text: I18n.t("notif.settleRequested", { actor, title }), href };
      case "bill.settle_rejected":
        return { text: I18n.t("notif.settleRejected", { title }), href };
      default:
        return null; // bill.updated is a silent refresh
    }
  }

  // --- dispatch -------------------------------------------------------------

  function on(type, fn) {
    if (!handlers.has(type)) handlers.set(type, []);
    handlers.get(type).push(fn);
  }

  function onAny(fn) {
    anyHandlers.push(fn);
  }

  function dispatch(event) {
    // One user may have several tabs open on different projects, so the hub
    // delivers to the user and the tab filters. An event with no project id is
    // treated as belonging to whichever project this tab is showing.
    if (event.project_id && typeof Chrome !== "undefined" && Chrome.currentProjectID) {
      const mine = Chrome.currentProjectID();
      if (mine && event.project_id !== mine) return;
    }

    // Record the notification first, so the bell updates even if a page hook is
    // slow or throws.
    const note = describe(event);
    if (note) add(event.type, note.text, note.href);

    const list = handlers.get(event.type) || [];
    for (const fn of list) {
      try {
        fn(event);
      } catch (err) {
        // A broken page hook must not stop the others from running.
        if (window.console) console.error("realtime handler failed", err);
      }
    }
    for (const fn of anyHandlers) {
      try {
        fn(event);
      } catch (err) {
        if (window.console) console.error("realtime handler failed", err);
      }
    }
  }

  // --- inbox ----------------------------------------------------------------

  // add records one notification and refreshes the bell. Ephemeral by design:
  // the list lives only as long as this page.
  function add(kind, text, href) {
    entries.unshift({ id: Date.now() + Math.random(), kind, text, href: href || "", at: new Date() });
    if (entries.length > MAX_ENTRIES) entries.length = MAX_ENTRIES;
    renderBell();
    if (document.hidden !== true) flash(kind);
  }

  function clear() {
    entries.length = 0;
    renderBell();
    closeTray();
  }

  // --- bell UI --------------------------------------------------------------

  let bellButton = null;
  let badge = null;
  let tray = null;
  let unread = 0;

  function mountBell() {
    const actions = document.querySelector(".topbar-actions");
    if (!actions || document.getElementById("notif-bell")) return;

    const wrap = UI.el("div", "notif-wrap");

    bellButton = document.createElement("button");
    bellButton.type = "button";
    bellButton.id = "notif-bell";
    bellButton.className = "icon-btn notif-bell";
    bellButton.dataset.icon = "bell";
    bellButton.setAttribute("aria-label", I18n.t("notif.open"));
    bellButton.setAttribute("aria-expanded", "false");
    bellButton.title = I18n.t("notif.open");

    badge = UI.el("span", "notif-badge hidden");
    bellButton.appendChild(badge);

    tray = UI.el("div", "notif-tray hidden");
    tray.setAttribute("role", "dialog");
    tray.setAttribute("aria-label", I18n.t("notif.title"));

    wrap.appendChild(bellButton);
    wrap.appendChild(tray);
    actions.insertBefore(wrap, actions.firstChild);

    Icons.apply(wrap);

    bellButton.addEventListener("click", (event) => {
      event.stopPropagation();
      toggleTray();
    });
    document.addEventListener("click", (event) => {
      if (!wrap.contains(event.target)) closeTray();
    });
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") closeTray();
    });

    renderBell();
  }

  function toggleTray() {
    if (tray.classList.contains("hidden")) {
      openTray();
      // Opening the tray counts as reading everything in it.
      unread = 0;
      renderBell();
    } else {
      closeTray();
    }
  }

  function openTray() {
    tray.classList.remove("hidden");
    bellButton.setAttribute("aria-expanded", "true");
  }

  function closeTray() {
    if (!tray) return;
    tray.classList.add("hidden");
    bellButton.setAttribute("aria-expanded", "false");
  }

  function renderBell() {
    if (!tray) return;

    if (unread > 0) {
      badge.textContent = unread > 9 ? "9+" : String(unread);
      badge.classList.remove("hidden");
    } else {
      badge.classList.add("hidden");
    }

    tray.replaceChildren();

    const head = UI.el("div", "notif-tray-head");
    head.appendChild(UI.el("strong", "", I18n.t("notif.title")));
    if (entries.length > 0) {
      const clearBtn = UI.el("button", "btn btn-quiet btn-sm", I18n.t("notif.clear"));
      clearBtn.type = "button";
      clearBtn.addEventListener("click", (event) => {
        event.stopPropagation();
        clear();
      });
      head.appendChild(clearBtn);
    }
    tray.appendChild(head);

    if (entries.length === 0) {
      tray.appendChild(UI.el("div", "notif-empty", I18n.t("notif.empty")));
      return;
    }

    const list = UI.el("div", "notif-list");
    for (const entry of entries) {
      const item = entry.href ? document.createElement("a") : document.createElement("div");
      item.className = "notif-item";
      if (entry.href) item.href = entry.href;
      item.appendChild(UI.el("div", "notif-text", entry.text));
      item.appendChild(UI.el("div", "notif-time", UI.formatDate(entry.at.toISOString().slice(0, 10)) +
        " · " + entry.at.toLocaleTimeString(I18n.getLocale(), { hour: "2-digit", minute: "2-digit" })));
      if (entry.href) {
        item.addEventListener("click", () => closeTray());
      }
      list.appendChild(item);
    }
    tray.appendChild(list);
  }

  // flash briefly highlights the bell so a new arrival is noticeable even when
  // the tray is closed.
  function flash(kind) {
    if (!bellButton) return;
    bellButton.classList.add("notif-flash");
    window.setTimeout(() => bellButton.classList.remove("notif-flash"), 900);
    if (kind !== "bill.updated") {
      unread += 1;
      renderBell();
    } else if (unread > 0) {
      renderBell();
    }
  }

  // --- connection -----------------------------------------------------------

  function url() {
    const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
    // The socket is established under the project this tab is showing. Events
    // for other projects are filtered out in dispatch.
    const projectId = typeof Chrome !== "undefined" && Chrome.currentProjectID
      ? Chrome.currentProjectID()
      : 0;
    return `${scheme}//${window.location.host}/api/p/${projectId}/ws`;
  }

  function connect() {
    if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
      return;
    }
    manualClose = false;

    try {
      socket = new WebSocket(url());
    } catch (err) {
      scheduleReconnect();
      return;
    }

    socket.addEventListener("open", () => {
      attempt = 0;
      // Pages that care can catch up on anything missed while disconnected.
      for (const fn of anyHandlers) {
        try {
          fn({ type: "socket.open" });
        } catch (err) {
          /* ignore */
        }
      }
    });

    socket.addEventListener("message", (event) => {
      let payload;
      try {
        payload = JSON.parse(event.data);
      } catch (err) {
        return;
      }
      if (payload && payload.type) dispatch(payload);
    });

    socket.addEventListener("close", () => {
      socket = null;
      if (!manualClose) scheduleReconnect();
    });

    socket.addEventListener("error", () => {
      // close fires next and handles reconnection.
    });
  }

  // scheduleReconnect backs off exponentially with jitter so a server restart
  // does not produce a thundering herd of clients.
  function scheduleReconnect() {
    if (reconnectTimer || manualClose) return;
    const base = Math.min(30000, 1000 * Math.pow(2, attempt));
    const delay = base * (0.7 + Math.random() * 0.6);
    attempt += 1;
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  }

  function start() {
    mountBell();
    connect();

    // Wake up immediately when the tab becomes visible again (covers sleep and
    // suspended background tabs) or when the network returns.
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "visible") connect();
    });
    window.addEventListener("online", () => connect());
  }

  return { start, on, onAny, add, clear, connect, describe, entries: () => entries.slice() };
})();
