(window.I18n ? I18n.init() : null);

const Chrome = (() => {
  function markActiveNav() {
    const here = window.location.pathname.replace(/\/$/, "") || "/";
    document.querySelectorAll("[data-nav]").forEach((link) => {
      const target = (link.getAttribute("href") || "").replace(/\/$/, "") || "/";
      if (target === here) {
        link.classList.add("active");
        link.setAttribute("aria-current", "page");
      }
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

  async function start(options) {
    const opts = options || {};
    try {
      await API.getCurrency();
    } catch (err) {
      if (err.status === 401) return;
    }
    markActiveNav();
    wireLogout();
    if (window.I18n) {
      I18n.onChange(() => rerender());
    }
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

  return { start };
})();
