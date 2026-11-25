const API = (() => {
  let currency = "$";

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
      } catch (err) {
        if (err.status === 401) throw err;
      }
      return currency;
    },

    currency() {
      return currency;
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

    updateProfile(displayName, username, email, language) {
      return request("PUT", "/api/profile", {
        display_name: displayName,
        username,
        email,
        language,
      });
    },

    changePassword(currentPassword, newPassword) {
      return request("PUT", "/api/profile/password", {
        current_password: currentPassword,
        new_password: newPassword,
      });
    },

    publicBills() {
      return request("GET", "/api/public/bills", undefined, { redirectOn401: false });
    },

    publicBill(id) {
      return request("GET", `/api/public/bills/${id}`, undefined, { redirectOn401: false });
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

    sendInvite(memberId) {
      return request("POST", `/api/members/${memberId}/invite`);
    },

    updateBillStatus(id, status) {
      return request("PUT", `/api/bills/${id}/status`, { status });
    },

    listMembers() {
      return request("GET", "/api/members");
    },

    getMember(id) {
      return request("GET", `/api/members/${id}`);
    },

    createMember(data) {
      return request("POST", "/api/members", data);
    },

    updateMember(id, data) {
      return request("PUT", `/api/members/${id}`, data);
    },

    listGroups() {
      return request("GET", "/api/groups");
    },

    getGroup(id) {
      return request("GET", `/api/groups/${id}`);
    },

    createGroup(data) {
      return request("POST", "/api/groups", data);
    },

    updateGroup(id, data) {
      return request("PUT", `/api/groups/${id}`, data);
    },

    listBills() {
      return request("GET", "/api/bills");
    },

    getBill(id) {
      return request("GET", `/api/bills/${id}`);
    },

    deleteBill(id) {
      return request("DELETE", `/api/bills/${id}`);
    },

    balances() {
      return request("GET", "/api/balances");
    },

    async createBill(payload, files) {
      const form = new FormData();
      form.append("payload", JSON.stringify(payload));
      for (const file of files || []) {
        form.append("files", file, file.name);
      }

      let response;
      try {
        response = await fetch("/api/bills", {
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
  function toCents(value) {
    if (typeof value === "bigint") return value;
    if (!Number.isFinite(value)) return 0;
    return Math.round(value * 100);
  }

  function format(cents, options) {
    const opts = options || {};
    const amount = typeof cents === "bigint" ? cents : BigInt(Math.trunc(Number(cents) || 0));
    const negative = amount < 0n;
    const abs = negative ? -amount : amount;
    const whole = abs / 100n;
    const fraction = abs % 100n;
    const wholeText = whole.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
    const body = `${wholeText}.${fraction.toString().padStart(2, "0")}`;
    const sign = negative ? "-" : opts.signed && abs > 0n ? "+" : "";
    return `${sign}${API.currency()}${body}`;
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
    return (amount / 100).toFixed(2);
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
