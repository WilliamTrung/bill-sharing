// `const I18n` is a global lexical binding, not a window property, so check the
// identifier itself rather than window.I18n.
(typeof I18n !== "undefined" ? I18n.init() : null);

const Chrome = (() => {
  // Current project is derived from the URL (/p/<id>/...). One tab can show a
  // different project from another, which is why it is not stored globally.
  function currentProjectID() {
    const match = window.location.pathname.match(/^\/p\/(\d+)(?:\/|$)/);
    return match ? Number(match[1]) : 0;
  }

  // projectURL builds an in-app link under the current project.
  function projectURL(path) {
    const id = currentProjectID();
    return id ? `/p/${id}${path}` : path;
  }

  // rewriteLinks points the shared header nav at the current project. The nav
  // markup is duplicated across pages, so doing it here keeps one source of
  // truth instead of editing every HTML file.
  function rewriteLinks() {
    const id = currentProjectID();
    if (!id) return;
    document.querySelectorAll("[data-nav], .brand, [data-project-link]").forEach((link) => {
      const href = link.getAttribute("href") || "";
      if (!href.startsWith("/") || href.startsWith("/p/") || href.startsWith("/share/")) return;
      link.setAttribute("href", `/p/${id}${href === "/" ? "/dashboard.html" : href}`);
    });
  }

  function markActiveNav() {
    const here = window.location.pathname.replace(/\/$/, "") || "/";
    document.querySelectorAll("[data-nav]").forEach((link) => {
      // Compare full paths, and also treat /admin and /admin/index.html as one.
      const target = (link.getAttribute("href") || "").replace(/\/$/, "").replace("/index.html", "") || "/";
      const hereNormalized = here.replace("/index.html", "");
      const active = target === hereNormalized;
      link.classList.toggle("active", active);
      if (active) {
        link.setAttribute("aria-current", "page");
      } else {
        link.removeAttribute("aria-current");
      }
    });
  }

  // mountNavToggle adds a hamburger that collapses the header nav on narrow
  // screens, so links never wrap onto a second line.
  function mountNavToggle() {
    document.querySelectorAll(".topbar-inner").forEach((bar) => {
      const nav = bar.querySelector(".nav");
      if (!nav || bar.querySelector(".nav-toggle")) return;

      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "nav-toggle";
      btn.setAttribute("aria-expanded", "false");
      btn.setAttribute("aria-controls", nav.id || "");
      btn.setAttribute("data-i18n-aria-label", "nav.toggle");
      btn.innerHTML = "<span></span><span></span><span></span>";

      const close = () => {
        nav.classList.remove("open");
        btn.setAttribute("aria-expanded", "false");
        btn.classList.remove("open");
      };

      btn.addEventListener("click", (event) => {
        event.stopPropagation();
        const open = !nav.classList.contains("open");
        nav.classList.toggle("open", open);
        btn.classList.toggle("open", open);
        btn.setAttribute("aria-expanded", String(open));
      });

      // Collapse when a link is followed or the user clicks elsewhere.
      nav.addEventListener("click", (event) => {
        if (event.target.closest("a")) close();
      });
      document.addEventListener("click", (event) => {
        if (!bar.contains(event.target)) close();
      });
      document.addEventListener("keydown", (event) => {
        if (event.key === "Escape") close();
      });

      nav.parentNode.insertBefore(btn, nav);
    });
  }

  function wireLogout() {
    document.querySelectorAll("[data-logout]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        btn.disabled = true;
        try {
          await API.logout();
        } catch (err) {
          /* cookie is cleared client-side regardless */
        }
        window.location.replace("/login.html");
      });
    });
  }

  function rerender() {
    markActiveNav();
    if (typeof Chrome.onLanguageChange === "function") {
      Chrome.onLanguageChange();
    }
  }

  // --- project pane ---------------------------------------------------------

  const PANE_KEY = "bs.pane.collapsed";

  // mountProjectPane injects the collapsible left pane listing every project
  // plus a create button. It is built here rather than in each HTML file because
  // the page shell is duplicated across six documents.
  function mountProjectPane(projects) {
    const main = document.querySelector("main.main");
    if (!main || document.getElementById("project-pane")) return;

    const currentId = currentProjectID();

    const aside = document.createElement("aside");
    aside.className = "project-pane";
    aside.id = "project-pane";
    aside.setAttribute("aria-label", I18n.t("projects.title"));

    const head = UI.el("div", "pane-head");
    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "pane-toggle icon-btn";
    toggle.dataset.icon = "chevron-right";
    toggle.setAttribute("aria-expanded", "false");
    toggle.setAttribute("aria-controls", "project-pane-list");
    toggle.title = I18n.t("projects.collapse");

    const title = UI.el("span", "pane-title", I18n.t("projects.title"));

    const addBtn = document.createElement("button");
    addBtn.type = "button";
    addBtn.className = "pane-add icon-btn";
    addBtn.dataset.icon = "plus";
    addBtn.title = I18n.t("projects.new");
    addBtn.setAttribute("aria-label", I18n.t("projects.new"));

    head.appendChild(toggle);
    head.appendChild(title);
    head.appendChild(addBtn);
    aside.appendChild(head);

    const list = UI.el("ul", "pane-list");
    list.id = "project-pane-list";
    if (projects.length === 0) {
      list.appendChild(UI.el("li", "pane-empty", I18n.t("projects.empty")));
    }
    for (const project of projects) {
      const item = document.createElement("li");
      const link = document.createElement("a");
      link.className = "pane-link";
      link.href = `/p/${project.id}/dashboard.html`;
      if (project.id === currentId) {
        link.classList.add("active");
        link.setAttribute("aria-current", "page");
      }
      link.appendChild(UI.el("span", "pane-name", project.name));
      if (project.role === "owner") {
        link.appendChild(UI.el("span", "pane-role", I18n.t("projects.owner")));
      }
      item.appendChild(link);
      list.appendChild(item);
    }
    aside.appendChild(list);

    // Wrap existing page content so the pane can sit beside it without touching
    // markup that pages already put directly inside <main>.
    const content = document.createElement("div");
    content.className = "main-content";
    while (main.firstChild) content.appendChild(main.firstChild);
    main.appendChild(aside);
    main.appendChild(content);
    main.classList.add("main-layout");

    Icons.apply(aside);

    const setCollapsed = (collapsed) => {
      document.body.classList.toggle("pane-collapsed", collapsed);
      toggle.setAttribute("aria-expanded", String(!collapsed));
      try {
        window.localStorage.setItem(PANE_KEY, collapsed ? "1" : "0");
      } catch (err) {
        /* private mode: the choice just does not persist */
      }
    };

    let stored = "0";
    try {
      stored = window.localStorage.getItem(PANE_KEY) || "0";
    } catch (err) {
      /* ignore */
    }
    if (stored === "1") setCollapsed(true);

    toggle.addEventListener("click", () => {
      setCollapsed(!document.body.classList.contains("pane-collapsed"));
    });

    addBtn.addEventListener("click", () => promptNewProject());

    // On narrow screens the pane is off-canvas; a tap outside closes it.
    document.addEventListener("click", (event) => {
      if (window.innerWidth > 820) return;
      if (!aside.contains(event.target) && !aside.classList.contains("hidden")) {
        aside.classList.remove("open");
      }
    });
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") aside.classList.remove("open");
    });

    if (window.innerWidth <= 820) {
      toggle.addEventListener("click", () => aside.classList.toggle("open"));
    }
  }

  // promptNewProject asks for a name and creates the project, then navigates
  // into it so the user lands somewhere useful.
  async function promptNewProject() {
    const name = await Popup.prompt({
      title: I18n.t("projects.new"),
      label: I18n.t("projects.newPrompt"),
      confirmLabel: I18n.t("action.create"),
      icon: "plus",
    });
    if (name === null) return;
    const trimmed = name.trim();
    if (!trimmed) return;
    try {
      const project = await API.createProject(trimmed);
      window.location.href = `/p/${project.id}/dashboard.html`;
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
    }
  }

  // startRealtime opens the event stream and forwards every event to the
  // current page's `Chrome.onRealtime` hook, if it has one. Pages that do not
  // opt in still get the notification bell.
  function startRealtime() {
    if (typeof Realtime === "undefined") return;
    Realtime.start();
    Realtime.onAny((event) => {
      if (typeof Chrome.onRealtime === "function") {
        Chrome.onRealtime(event);
      }
    });
  }

  async function start(options) {
    const opts = options || {};
    try {
      await API.getCurrency();
    } catch (err) {
      if (err.status === 401) return;
    }
    let authenticated = false;
    let session = null;
    if (typeof I18n !== "undefined") {
      session = await API.currentSession();
      authenticated = session !== null;
      if (session && session.language) {
        I18n.useDefault(session.language);
        I18n.apply();
      }
      I18n.onChange(() => rerender());
    }
    rewriteLinks();
    markActiveNav();
    mountNavToggle();
    wireLogout();
    Icons.apply();
    // The pane needs the project list, which the session already carries.
    if (authenticated && session) {
      mountProjectPane(session.projects || []);
    }
    // Only signed-in pages get the bell and the event stream; the public share
    // page is anonymous and its socket would be rejected anyway.
    if (authenticated) startRealtime();
    if (opts.onReady) {
      try {
        await opts.onReady();
      } catch (err) {
        if (err && err.status === 401) return;
        UI.toast(UI.errorMessage(err), "error");
        if (opts.onError) opts.onError(err);
      }
    }
  }

  return { start, currentProjectID, projectURL };
})();

const Rows = (() => {
  // makeClickable turns a table row into a link on the whole surface, while
  // leaving controls inside it (links, buttons, the shares toggle) to themselves.
  function makeClickable(row) {
    const href = row.dataset.href;
    if (!href) return;

    const go = () => window.location.assign(href);
    row.addEventListener("click", (event) => {
      if (event.target.closest("a, button, input, select, textarea, label")) return;
      go();
    });
    row.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" || event.target !== row) return;
      event.preventDefault();
      go();
    });
  }

  function apply(root) {
    (root || document).querySelectorAll("tr[data-href]").forEach(makeClickable);
  }

  return { apply, makeClickable };
})();

const Icons = (() => {
  // 24x24 stroke icons; they inherit currentColor so they work on any button.
  const paths = {
    plus: '<path d="M12 5v14M5 12h14"/>',
    users:
      '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>' +
      '<path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
    user: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
    mail: '<rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/>',
    edit: '<path d="M17 3a2.85 2.85 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    "check-circle": '<circle cx="12" cy="12" r="9"/><path d="m8.5 12.5 2.5 2.5 4.5-5"/>',
    x: '<path d="M18 6 6 18M6 6l12 12"/>',
    trash: '<path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/>',
    "arrow-left": '<path d="M19 12H5M12 19l-7-7 7-7"/>',
    "log-out": '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="m16 17 5-5-5-5M21 12H9"/>',
    "log-in": '<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><path d="m10 17 5-5-5-5M15 12H3"/>',
    save: '<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2Z"/><path d="M17 21v-8H7v8M7 3v5h8"/>',
    key: '<circle cx="7.5" cy="15.5" r="4.5"/><path d="m10.7 12.3 8.3-8.3M17 6l2 2M14 9l2 2"/>',
    eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>',
    settings:
      '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z"/>',
    undo: '<path d="M3 7v6h6"/><path d="M3 13a9 9 0 1 0 3-7.7L3 8"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
    "chevron-right": '<path d="m9 18 6-6-6-6"/>',
    bell:
      '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/>' +
      '<path d="M13.7 21a2 2 0 0 1-3.4 0"/>',
  };

  function svg(name, size) {
    const body = paths[name];
    if (!body) return null;
    const el = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    el.setAttribute("viewBox", "0 0 24 24");
    el.setAttribute("width", String(size || 16));
    el.setAttribute("height", String(size || 16));
    el.setAttribute("fill", "none");
    el.setAttribute("stroke", "currentColor");
    el.setAttribute("stroke-width", "2");
    el.setAttribute("stroke-linecap", "round");
    el.setAttribute("stroke-linejoin", "round");
    el.setAttribute("aria-hidden", "true");
    el.setAttribute("focusable", "false");
    el.innerHTML = body;
    return el;
  }

  // Decorate every [data-icon] element with its icon, keeping the label text.
  function apply(root) {
    (root || document).querySelectorAll("[data-icon]").forEach((node) => {
      if (node.dataset.iconReady === "true") return;
      const icon = svg(node.dataset.icon);
      if (!icon) return;
      node.dataset.iconReady = "true";
      node.classList.add("has-icon");
      node.insertBefore(icon, node.firstChild);
    });
  }

  // setLabel swaps a button's text without discarding the icon Icons.apply added.
  function setLabel(node, text) {
    const label = node.querySelector("[data-label]");
    if (label) {
      label.textContent = text;
      return;
    }
    const icon = node.querySelector("svg");
    node.replaceChildren();
    if (icon) node.appendChild(icon);
    node.appendChild(document.createTextNode(text));
  }

  return { svg, apply, setLabel };
})();

const Shares = (() => {
  function buildTable(bill) {
    const shares = bill.member_shares || [];
    const groupNames = new Map();
    for (const participant of bill.participants || []) {
      if (participant.participant_type === "GROUP" && participant.group_id) {
        groupNames.set(participant.group_id, participant.name);
      }
    }

    if (shares.length === 0) {
      return UI.el("div", "empty", I18n.t("shares.none"));
    }

    // Participant notes belong to top-level participants, so index them by member
    // id (and by group id for rows that came from a group).
    const notes = new Map();
    for (const participant of bill.participants || []) {
      if (!participant.note) continue;
      if (participant.member_id) notes.set(`m${participant.member_id}`, participant.note);
      if (participant.group_id) notes.set(`g${participant.group_id}`, participant.note);
    }

    const table = document.createElement("table");
    table.className = "table";

    const head = document.createElement("thead");
    const headRow = document.createElement("tr");
    headRow.appendChild(UI.el("th", "", I18n.t("shares.member")));
    headRow.appendChild(UI.el("th", "", I18n.t("shares.source")));
    headRow.appendChild(UI.el("th", "right", I18n.t("shares.amount")));
    headRow.appendChild(UI.el("th", "right", I18n.t("detail.settle")));
    head.appendChild(headRow);
    table.appendChild(head);

    const body = document.createElement("tbody");
    let total = 0n;
    for (const share of shares) {
      const row = document.createElement("tr");

      const memberCell = document.createElement("td");
      const person = UI.el("div", "person");
      person.appendChild(UI.avatar(share.member_name, null));
      const who = UI.el("div", "who");
      who.appendChild(UI.el("div", "name", share.member_name));
      const note = share.source_group_id
        ? notes.get(`g${share.source_group_id}`)
        : notes.get(`m${share.member_id}`);
      if (note) who.appendChild(UI.el("div", "sub", note));
      person.appendChild(who);
      memberCell.appendChild(person);
      row.appendChild(memberCell);

      const sourceCell = UI.el("td", "small muted");
      if (share.source_group_id) {
        const name = groupNames.get(share.source_group_id) || `#${share.source_group_id}`;
        sourceCell.textContent = I18n.t("detail.fromGroup", { group: name });
      } else {
        sourceCell.textContent = I18n.t("detail.direct");
      }
      row.appendChild(sourceCell);

      row.appendChild(UI.el("td", "right num strong", Money.format(share.owed_amount_cents)));
      row.appendChild(
        UI.el(
          "td",
          "right",
          I18n.t(
            share.is_payer
              ? "detail.payerPaid"
              : share.is_settled
                ? "detail.shareSettled"
                : "detail.shareOpen"
          )
        )
      );
      total += BigInt(share.owed_amount_cents);
      body.appendChild(row);
    }
    table.appendChild(body);

    const wrap = document.createElement("div");
    wrap.appendChild(table);
    const totalRow = UI.el("div", "preview-total");
    totalRow.appendChild(UI.el("span", "", I18n.t("detail.sumShares")));
    totalRow.appendChild(UI.el("span", "num", Money.format(total)));
    wrap.appendChild(totalRow);
    return wrap;
  }

  function mount(toggleCell, row, colspan, billId, loader, preloadedShares) {
    // The dropdown row is inserted beside this one, so it must already be attached.
    if (!row.parentNode) return;

    const detailRow = document.createElement("tr");
    detailRow.className = "shares-row hidden";
    const detailCell = document.createElement("td");
    detailCell.colSpan = colspan;
    detailRow.appendChild(detailCell);
    row.parentNode.insertBefore(detailRow, row.nextSibling);

    const button = document.createElement("button");
    button.type = "button";
    button.className = "btn btn-quiet btn-sm bill-toggle";
    button.setAttribute("aria-expanded", "false");
    button.title = I18n.t("shares.show");
    button.textContent = "▸";
    toggleCell.appendChild(button);

    // Shares already attached to the list response avoid an extra request.
    let loaded = false;
    if (Array.isArray(preloadedShares) && preloadedShares.length > 0) {
      detailCell.replaceChildren(buildTable({ member_shares: preloadedShares }));
      loaded = true;
    }
    let expanded = false;

    function setExpanded(next) {
      expanded = next;
      detailRow.classList.toggle("hidden", !expanded);
      button.setAttribute("aria-expanded", String(expanded));
      button.title = I18n.t(expanded ? "shares.hide" : "shares.show");
      button.textContent = expanded ? "▾" : "▸";
      row.classList.toggle("expanded", expanded);
    }

    button.addEventListener("click", async () => {
      if (expanded) {
        setExpanded(false);
        return;
      }
      if (!loaded) {
        button.disabled = true;
        try {
          detailCell.replaceChildren(buildTable(await loader(billId)));
          loaded = true;
        } catch (err) {
          detailCell.replaceChildren(UI.el("div", "notice error", UI.errorMessage(err)));
        } finally {
          button.disabled = false;
        }
      }
      setExpanded(true);
    });
  }

  return { mount, buildTable };
})();
