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

    const table = document.createElement("table");
    table.className = "table";

    const head = document.createElement("thead");
    const headRow = document.createElement("tr");
    headRow.appendChild(UI.el("th", "", I18n.t("shares.member")));
    headRow.appendChild(UI.el("th", "", I18n.t("shares.source")));
    headRow.appendChild(UI.el("th", "right", I18n.t("shares.amount")));
    head.appendChild(headRow);
    table.appendChild(head);

    const body = document.createElement("tbody");
    let total = 0n;
    for (const share of shares) {
      const row = document.createElement("tr");

      const memberCell = document.createElement("td");
      const person = UI.el("div", "person");
      person.appendChild(UI.avatar(share.member_name, null));
      person.appendChild(UI.el("span", "strong", share.member_name));
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

  function mount(toggleCell, row, colspan, billId, loader) {
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

    let loaded = false;
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
