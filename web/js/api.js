const API = (() => {
  let currency = "$";
  let decimals = 2;

  class ApiError extends Error {
    constructor(message, status) {
      super(message);
      this.name = "ApiError";
      this.status = status;
    }
  }

  function tr(key, vars) {
    return typeof I18n !== "undefined" ? I18n.t(key, vars) : key;
  }

  // pp builds a project-scoped API path, so the prefix lives in exactly one place.
  function pp(projectId, path) {
    return `/api/p/${projectId}${path}`;
  }

  function serverMessage(raw, fallbackKey, status) {
    const translated = typeof I18n !== "undefined" ? I18n.translateServer(raw) : raw;
    if (!raw) return tr(fallbackKey, { status });
    return translated && translated !== raw ? translated : raw;
  }

  async function request(method, path, body, options) {
    const opts = options || {};
    const redirectOn401 = opts.redirectOn401 !== false;
    const req = { method, credentials: "same-origin", headers: {} };
    if (body !== undefined) {
      req.headers["Content-Type"] = "application/json";
      req.body = JSON.stringify(body);
    }

    let response;
    try {
      response = await fetch(path, req);
    } catch (err) {
      throw new ApiError(tr("msg.networkError"), 0);
    }

    if (response.status === 401 && !path.startsWith("/api/login")) {
      if (redirectOn401) redirectToLogin();
      throw new ApiError(tr("msg.sessionExpired"), 401);
    }

    let payload = null;
    const contentType = response.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
      try {
        payload = await response.json();
      } catch (err) {
        payload = null;
      }
    }

    if (!response.ok) {
      const message = serverMessage(payload && payload.error, "msg.requestFailed", response.status);
      throw new ApiError(message, response.status);
    }

    return payload;
  }

  function redirectToLogin() {
    const path = window.location.pathname.replace(/\/$/, "") || "/";
    if (path !== "/login.html" && path !== "/") {
      window.location.replace("/login.html");
    }
  }

  return {
    ApiError,

    async getCurrency() {
      try {
        const cfg = await request("GET", "/api/config");
        if (cfg && typeof cfg.currency === "string" && cfg.currency.length <= 4) {
          currency = cfg.currency;
        }
        if (cfg && Number.isInteger(cfg.currency_decimals) && cfg.currency_decimals >= 0 && cfg.currency_decimals <= 4) {
          decimals = cfg.currency_decimals;
        }
      } catch (err) {
        if (err.status === 401) throw err;
      }
      return currency;
    },

    currency() {
      return currency;
    },

    digits() {
      return decimals;
    },

    login(identifier, password) {
      let id = "";
      let secret = "";
      if (identifier && typeof identifier === "object") {
        id = String(identifier.identifier ?? identifier.username ?? identifier.email ?? "").trim();
        secret = String(identifier.password ?? "");
      } else {
        id = String(identifier ?? "").trim();
        secret = String(password ?? "");
      }
      if (!id || !secret) {
        return Promise.reject(new ApiError(tr("login.errRequired"), 400));
      }
      return request("POST", "/api/login", { identifier: id, password: secret });
    },

    logout() {
      return request("POST", "/api/logout");
    },

    session() {
      return request("GET", "/api/session");
    },

    async currentSession() {
      try {
        return await request("GET", "/api/session", undefined, { redirectOn401: false });
      } catch (err) {
        return null;
      }
    },

    profile() {
      return request("GET", "/api/profile");
    },

    updateProfile(displayName, username, email, language, isPublic) {
      return request("PUT", "/api/profile", {
        display_name: displayName,
        username,
        email,
        language,
        is_public: isPublic === true,
      });
    },

    changePassword(currentPassword, newPassword) {
      return request("PUT", "/api/profile/password", {
        current_password: currentPassword,
        new_password: newPassword,
      });
    },

    // --- projects -----------------------------------------------------------

    listProjects() {
      return request("GET", "/api/projects");
    },

    createProject(name) {
      return request("POST", "/api/projects", { name });
    },

    getProject(projectId) {
      return request("GET", pp(projectId, ""));
    },

    updateProject(projectId, data) {
      return request("PUT", pp(projectId, ""), data);
    },

    deleteProject(projectId) {
      return request("DELETE", pp(projectId, ""));
    },

    listProjectMembers(projectId) {
      return request("GET", pp(projectId, "/members"));
    },

    addProjectMember(projectId, email, name) {
      return request("POST", pp(projectId, "/members"), { email, name: name || "" });
    },

    removeProjectMember(projectId, userId) {
      return request("DELETE", pp(projectId, `/members/${userId}`));
    },

    // --- public share links -------------------------------------------------

    publicProject(slug) {
      return request("GET", `/api/public/p/${encodeURIComponent(slug)}`, undefined, {
        redirectOn401: false,
      });
    },

    publicBills(slug) {
      return request("GET", `/api/public/p/${encodeURIComponent(slug)}/bills`, undefined, {
        redirectOn401: false,
      });
    },

    publicBill(slug, id) {
      return request("GET", `/api/public/p/${encodeURIComponent(slug)}/bills/${id}`, undefined, {
        redirectOn401: false,
      });
    },

    getInvite(token) {
      return request("GET", `/api/public/invite?token=${encodeURIComponent(token)}`, undefined, {
        redirectOn401: false,
      });
    },

    acceptInvite(token, password) {
      return request("POST", `/api/public/invite/accept?token=${encodeURIComponent(token)}`, { password }, {
        redirectOn401: false,
      });
    },

    // --- project-scoped data ------------------------------------------------

    sendInvite(projectId, userId) {
      return request("POST", pp(projectId, `/members/${userId}/invite`));
    },

    updateBillStatus(projectId, id, status) {
      return request("PUT", pp(projectId, `/bills/${id}/status`), { status });
    },

    updateShareSettled(projectId, billId, shareId, action) {
      return request("PUT", pp(projectId, `/bills/${billId}/shares/${shareId}/settled`), { action });
    },

    settlements(projectId) {
      return request("GET", pp(projectId, "/settlements"));
    },

    listMembers(projectId) {
      return request("GET", pp(projectId, "/people"));
    },

    getMember(projectId, id) {
      return request("GET", pp(projectId, `/people/${id}`));
    },

    createMember(projectId, data) {
      return request("POST", pp(projectId, "/people"), data);
    },

    updateMember(projectId, id, data) {
      return request("PUT", pp(projectId, `/people/${id}`), data);
    },

    listGroups(projectId) {
      return request("GET", pp(projectId, "/groups"));
    },

    getGroup(projectId, id) {
      return request("GET", pp(projectId, `/groups/${id}`));
    },

    createGroup(projectId, data) {
      return request("POST", pp(projectId, "/groups"), data);
    },

    updateGroup(projectId, id, data) {
      return request("PUT", pp(projectId, `/groups/${id}`), data);
    },

    listBills(projectId) {
      return request("GET", pp(projectId, "/bills"));
    },

    getBill(projectId, id) {
      return request("GET", pp(projectId, `/bills/${id}`));
    },

    deleteBill(projectId, id) {
      return request("DELETE", pp(projectId, `/bills/${id}`));
    },

    balances(projectId) {
      return request("GET", pp(projectId, "/balances"));
    },

    async createBill(projectId, payload, files) {
      const form = new FormData();
      form.append("payload", JSON.stringify(payload));
      for (const file of files || []) {
        form.append("files", file, file.name);
      }

      let response;
      try {
        response = await fetch(pp(projectId, "/bills"), {
          method: "POST",
          body: form,
          credentials: "same-origin",
        });
      } catch (err) {
        throw new ApiError(tr("msg.networkError"), 0);
      }

      if (response.status === 401) {
        redirectToLogin();
        throw new ApiError(tr("msg.sessionExpired"), 401);
      }

      let body = null;
      try {
        body = await response.json();
      } catch (err) {
        body = null;
      }
      if (!response.ok) {
        const message = serverMessage(body && body.error, "msg.uploadFailed", response.status);
        throw new ApiError(message, response.status);
      }
      return body;
    },
  };
})();

const Money = (() => {
  // Minor units per major unit, e.g. 100 for "$" (cents) or 1 for VND.
  function scale() {
    return 10 ** API.digits();
  }

  function toCents(value) {
    if (typeof value === "bigint") return value;
    if (!Number.isFinite(value)) return 0;
    return Math.round(value * scale());
  }

  function format(cents, options) {
    const opts = options || {};
    const digits = API.digits();
    const factor = 10n ** BigInt(digits);
    const amount = typeof cents === "bigint" ? cents : BigInt(Math.trunc(Number(cents) || 0));
    const negative = amount < 0n;
    const abs = negative ? -amount : amount;
    const whole = abs / factor;
    const fraction = abs % factor;
    const wholeText = whole.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
    const fractionText = digits > 0 ? `.${fraction.toString().padStart(digits, "0")}` : "";
    const sign = negative ? "-" : opts.signed && abs > 0n ? "+" : "";
    return `${sign}${API.currency()}${wholeText}${fractionText}`;
  }

  function parse(input) {
    if (typeof input === "number") return toCents(input);
    if (typeof input !== "string") return 0;
    const cleaned = input.replace(/[^0-9.\-]/g, "");
    if (cleaned === "" || cleaned === "-" || cleaned === ".") return 0;
    const value = Number.parseFloat(cleaned);
    if (!Number.isFinite(value)) return 0;
    return toCents(value);
  }

  function toInput(cents) {
    const amount = Number(cents) || 0;
    return (amount / scale()).toFixed(API.digits());
  }

  return { toCents, parse, format, toInput };
})();

const UI = (() => {
  let toastEl = null;
  let toastTimer = null;

  function ensureToast() {
    if (toastEl) return toastEl;
    toastEl = document.createElement("div");
    toastEl.className = "toast";
    toastEl.setAttribute("role", "status");
    toastEl.setAttribute("aria-live", "polite");
    document.body.appendChild(toastEl);
    return toastEl;
  }

  function toast(message, kind) {
    const el = ensureToast();
    el.textContent = message;
    el.className = `toast show ${kind || ""}`.trim();
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      el.className = "toast";
    }, 4200);
  }

  function initials(name) {
    const parts = String(name || "")
      .trim()
      .split(/\s+/)
      .filter(Boolean);
    if (parts.length === 0) return "?";
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }

  function avatar(name, color, size) {
    const el = document.createElement("span");
    el.className = "avatar";
    el.textContent = initials(name);
    el.setAttribute("aria-hidden", "true");
    el.style.background = color || "#6c6f6d";
    if (size) {
      el.style.width = `${size}px`;
      el.style.height = `${size}px`;
    }
    return el;
  }

  function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = text;
    return node;
  }

  function escapeHTML(value) {
    return String(value === undefined || value === null ? "" : value).replace(/[&<>"']/g, (ch) => {
      switch (ch) {
        case "&":
          return "&amp;";
        case "<":
          return "&lt;";
        case ">":
          return "&gt;";
        case '"':
          return "&quot;";
        case "'":
          return "&#39;";
        default:
          return ch;
      }
    });
  }

  function isActive(entity) {
    return entity && entity.is_active !== false;
  }

  function formatDate(iso) {
    if (!iso) return "";
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return iso;
    const locale = typeof I18n !== "undefined" ? I18n.getLocale() : undefined;
    return date.toLocaleDateString(locale, { year: "numeric", month: "short", day: "numeric" });
  }

  function todayISO() {
    const now = new Date();
    const month = String(now.getMonth() + 1).padStart(2, "0");
    const day = String(now.getDate()).padStart(2, "0");
    return `${now.getFullYear()}-${month}-${day}`;
  }

  function errorMessage(err) {
    if (err && err.message) return err.message;
    return typeof I18n !== "undefined" ? I18n.t("msg.somethingWrong") : "Something went wrong";
  }

  function debounce(fn, wait) {
    let timer = null;
    return (...args) => {
      clearTimeout(timer);
      timer = setTimeout(() => fn(...args), wait);
    };
  }

  return { toast, avatar, initials, el, escapeHTML, isActive, formatDate, todayISO, errorMessage, debounce };
})();

// In-app replacement for the browser's alert/confirm/prompt dialogs. It reuses
// the .modal shell, so the pane looks like the rest of the app, and the action
// buttons are icon-only (check/x/trash) with their label exposed as a tooltip.
const Popup = (() => {
  let overlay = null;
  let card = null;
  let nodes = null;
  let settle = null;
  let mode = "confirm";
  let previousFocus = null;

  function t(key) {
    return typeof I18n !== "undefined" ? I18n.t(key) : key;
  }

  function build() {
    if (overlay) return;

    overlay = UI.el("div", "modal popup");
    overlay.setAttribute("role", "dialog");
    overlay.setAttribute("aria-modal", "true");
    overlay.setAttribute("aria-labelledby", "popup-title");

    card = UI.el("div", "modal-card popup-card");

    const head = UI.el("div", "modal-head popup-head");
    const titles = UI.el("div", "popup-titles");
    const title = UI.el("h2", "popup-title");
    title.id = "popup-title";
    const message = UI.el("div", "popup-message muted");
    titles.appendChild(title);
    titles.appendChild(message);

    const closeBtn = UI.el("button", "icon-btn");
    closeBtn.type = "button";
    closeBtn.dataset.icon = "x";
    closeBtn.addEventListener("click", cancel);
    head.appendChild(titles);
    head.appendChild(closeBtn);

    const body = UI.el("div", "modal-body popup-body");
    const field = UI.el("div", "field popup-field");
    const label = document.createElement("label");
    label.setAttribute("for", "popup-input");
    const input = document.createElement("input");
    input.id = "popup-input";
    input.type = "text";
    input.autocomplete = "off";
    input.maxLength = 200;
    field.appendChild(label);
    field.appendChild(input);
    body.appendChild(field);

    const foot = UI.el("div", "modal-foot popup-foot");
    const cancelBtn = UI.el("button", "btn btn-ghost btn-icon");
    cancelBtn.type = "button";
    cancelBtn.dataset.icon = "x";
    cancelBtn.addEventListener("click", cancel);

    const confirmBtn = UI.el("button", "btn btn-icon");
    confirmBtn.type = "button";
    confirmBtn.dataset.icon = "check";
    confirmBtn.addEventListener("click", submit);

    foot.appendChild(cancelBtn);
    foot.appendChild(confirmBtn);

    card.appendChild(head);
    card.appendChild(body);
    card.appendChild(foot);
    overlay.appendChild(card);
    document.body.appendChild(overlay);

    nodes = { title, message, field, label, input, cancelBtn, confirmBtn, closeBtn };

    overlay.addEventListener("click", (event) => {
      if (event.target === overlay) cancel();
    });
    document.addEventListener("keydown", (event) => {
      if (!overlay.classList.contains("open")) return;
      if (event.key === "Escape") {
        event.preventDefault();
        cancel();
      } else if (event.key === "Enter" && event.target === input) {
        event.preventDefault();
        submit();
      }
    });
  }

  function decorate(button, name) {
    button.dataset.icon = name;
    delete button.dataset.iconReady;
    const svg = button.querySelector("svg");
    if (svg) svg.remove();
  }

  function setLabel(button, label) {
    button.setAttribute("aria-label", label);
    button.title = label;
  }

  function submit() {
    if (!settle) return;
    finish(mode === "prompt" ? nodes.input.value : true);
  }

  function cancel() {
    if (!settle) return;
    finish(mode === "prompt" ? null : false);
  }

  function finish(value) {
    if (!settle) return;
    const done = settle;
    settle = null;
    overlay.classList.remove("open");
    if (previousFocus && typeof previousFocus.focus === "function") {
      previousFocus.focus();
    }
    previousFocus = null;
    done(value);
  }

  function show(options) {
    build();
    const opts = options || {};
    mode = opts.mode || "confirm";
    const isPrompt = mode === "prompt";

    nodes.title.textContent = opts.title || "";
    nodes.message.textContent = opts.message || "";
    nodes.message.classList.toggle("hidden", !opts.message);

    nodes.field.classList.toggle("hidden", !isPrompt);
    nodes.label.textContent = opts.label || "";
    nodes.input.value = opts.value || "";
    nodes.input.placeholder = opts.placeholder || "";

    const confirmLabel = opts.confirmLabel || t("action.confirm");
    decorate(nodes.confirmBtn, opts.icon || "check");
    decorate(nodes.cancelBtn, "x");
    decorate(nodes.closeBtn, "x");
    nodes.confirmBtn.className = `btn btn-icon ${opts.tone === "danger" ? "btn-danger" : ""}`.trim();
    setLabel(nodes.confirmBtn, confirmLabel);
    setLabel(nodes.cancelBtn, t("action.cancel"));
    setLabel(nodes.closeBtn, t("action.close"));
    if (typeof Icons !== "undefined") Icons.apply(card);

    previousFocus = document.activeElement;
    overlay.classList.add("open");

    return new Promise((resolve) => {
      settle = resolve;
      const focusTarget = isPrompt ? nodes.input : nodes.confirmBtn;
      requestAnimationFrame(() => focusTarget.focus());
    });
  }

  function confirm(options) {
    return show({ ...options, mode: "confirm" }).then((result) => result === true);
  }

  function prompt(options) {
    return show({ ...options, mode: "prompt" }).then((result) =>
      typeof result === "string" ? result : null
    );
  }

  return { confirm, prompt };
})();
