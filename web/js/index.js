(() => {
  const summary = document.getElementById("summary");
  const balancesBody = document.getElementById("balances-body");
  const balancesEmpty = document.getElementById("balances-empty");
  const billsBody = document.getElementById("bills-body");
  const billsEmpty = document.getElementById("bills-empty");

  const cache = { bills: [], balances: [], settlements: [] };

  function unit(key, singularKey, count) {
    return I18n.t(count === 1 ? singularKey : key);
  }

  function stat(label, value, meta, tone) {
    const card = UI.el("div", "stat");
    card.appendChild(UI.el("div", "label", label));
    const valueEl = UI.el("div", `value ${tone || ""}`.trim(), value);
    card.appendChild(valueEl);
    if (meta) card.appendChild(UI.el("div", "meta", meta));
    return card;
  }

  function renderSummary(bills, balances) {
    summary.replaceChildren();

    let totalSpent = 0n;
    for (const bill of bills) {
      totalSpent += BigInt(bill.total_amount_cents || 0);
    }

    const activeMembers = balances.filter((b) => b.is_active).length;
    const creditors = balances.filter((b) => BigInt(b.net_cents) > 0n).length;
    const debtors = balances.filter((b) => BigInt(b.net_cents) < 0n).length;

    summary.appendChild(
      stat(
        I18n.t("index.totalTracked"),
        Money.format(totalSpent),
        `${bills.length} ${unit("unit.bills", "unit.bill", bills.length)}`
      )
    );
    summary.appendChild(
      stat(I18n.t("index.activeMembers"), String(activeMembers), `${balances.length} ${I18n.t("index.total")}`)
    );
    summary.appendChild(
      stat(
        I18n.t("index.balancesStat"),
        I18n.t("index.creditorsOwing", { creditors, debtors }),
        I18n.t("index.balancesSee")
      )
    );
  }

  function renderBalances(balances) {
    balancesBody.replaceChildren();
    // Your own row is not listed; the summary above still counts everyone.
    const others = balances.filter((b) => b.is_self !== true);
    if (others.length === 0) {
      balancesEmpty.classList.remove("hidden");
      return;
    }
    balancesEmpty.classList.add("hidden");

    for (const balance of others) {
      const row = document.createElement("tr");

      const memberCell = document.createElement("td");
      const person = UI.el("div", "person");
      person.appendChild(UI.avatar(balance.name, null));
      const who = UI.el("div", "who");
      who.appendChild(UI.el("div", "name", balance.name));
      if (!balance.is_active) who.appendChild(UI.el("div", "sub", I18n.t("state.inactive")));
      person.appendChild(who);
      memberCell.appendChild(person);
      row.appendChild(memberCell);

      const net = BigInt(balance.net_cents);
      const netCell = UI.el("td", "right num strong");
      netCell.textContent = Money.format(net, { signed: true });
      if (net > 0n) netCell.style.color = "var(--accent-strong)";
      if (net < 0n) netCell.style.color = "var(--danger)";
      row.appendChild(netCell);

      balancesBody.appendChild(row);
    }
  }

  function renderBills(bills) {
    billsBody.replaceChildren();
    if (bills.length === 0) {
      billsEmpty.classList.remove("hidden");
      return;
    }
    billsEmpty.classList.add("hidden");

    for (const bill of bills) {
      const row = document.createElement("tr");
      row.className = "bill-row";
      row.dataset.href = `/bill-detail.html?id=${bill.id}`;
      row.tabIndex = 0;
      row.setAttribute("role", "link");

      const toggleCell = document.createElement("td");
      // Keep the shares toggle from triggering the row navigation.
      toggleCell.addEventListener("click", (event) => event.stopPropagation());
      row.appendChild(toggleCell);

      const titleCell = document.createElement("td");
      const link = document.createElement("a");
      link.href = `/bill-detail.html?id=${bill.id}`;
      link.className = "strong";
      link.textContent = bill.title;
      titleCell.appendChild(link);
      const meta = [];
      if (bill.description) meta.push(bill.description);
      if (bill.attachment_count) {
        meta.push(`${bill.attachment_count} ${unit("unit.attachments", "unit.attachment", bill.attachment_count)}`);
      }
      if (bill.participant_count) {
        meta.push(`${bill.participant_count} ${unit("unit.participants", "unit.participant", bill.participant_count)}`);
      }
      if (meta.length) titleCell.appendChild(UI.el("div", "small faint", meta.join(" · ")));
      row.appendChild(titleCell);

      row.appendChild(UI.el("td", "small", UI.formatDate(bill.bill_date)));
      row.appendChild(UI.el("td", "small", bill.paid_by_name));

      const isRatio = bill.split_mode === "RATIO";
      const splitPill = UI.el("span", `pill ${isRatio ? "pill-warn" : "pill-accent"}`, I18n.t(isRatio ? "index.splitRatio" : "index.splitEven"));
      const splitCell = document.createElement("td");
      splitCell.appendChild(splitPill);
      row.appendChild(splitCell);

      const settled = bill.status === "SETTLED";
      const statusCell = document.createElement("td");
      statusCell.appendChild(
        UI.el(
          "span",
          `pill ${settled ? "pill-warn" : "pill-accent"}`,
          I18n.t(settled ? "public.statusSettled" : "public.statusOpen")
        )
      );
      row.appendChild(statusCell);

      row.appendChild(UI.el("td", "right num strong", Money.format(bill.total_amount_cents)));

      billsBody.appendChild(row);
      Shares.mount(toggleCell, row, 7, bill.id, API.getBill);
    }
    Rows.apply(billsBody);
  }

  const settleBody = document.getElementById("settle-body");
  const settleEmpty = document.getElementById("settle-empty");

  function iconButton(icon, labelKey, onClick) {
    const label = I18n.t(labelKey);
    const button = UI.el("button", "btn btn-quiet btn-sm btn-icon");
    button.type = "button";
    button.dataset.icon = icon;
    button.title = label;
    button.setAttribute("aria-label", label);
    button.addEventListener("click", onClick);
    return button;
  }

  async function actOnShare(item, action) {
    if (action === "settle") {
      const ok = window.confirm(
        I18n.t("detail.confirmSettle", { name: item.counterparty_name, amount: Money.format(item.owed_amount_cents) })
      );
      if (!ok) return;
    }
    try {
      await API.updateShareSettled(item.bill_id, item.share_id, action);
      UI.toast(I18n.t("index.settleUpdated"), "success");
      cache.settlements = await API.settlements();
      renderSettlements(cache.settlements);
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
    }
  }

  function renderSettlements(items) {
    settleBody.replaceChildren();
    if (!items || items.length === 0) {
      settleEmpty.classList.remove("hidden");
      return;
    }
    settleEmpty.classList.add("hidden");

    for (const item of items) {
      const row = document.createElement("tr");
      row.dataset.href = `/bill-detail.html?id=${item.bill_id}`;

      const titleCell = document.createElement("td");
      titleCell.appendChild(UI.el("div", "strong", item.bill_title));
      const meta = [UI.formatDate(item.bill_date)];
      if (item.note) meta.push(item.note);
      titleCell.appendChild(UI.el("div", "small faint", meta.join(" · ")));
      row.appendChild(titleCell);

      row.appendChild(UI.el("td", "small", item.counterparty_name));
      row.appendChild(
        UI.el(
          "td",
          "small",
          I18n.t(item.role === "payer" ? "index.rolePayer" : "index.roleDebtor")
        )
      );
      row.appendChild(UI.el("td", "right num strong", Money.format(item.owed_amount_cents)));

      const actionCell = document.createElement("td");
      actionCell.className = "right";
      // Keep these controls from triggering the row's navigation.
      actionCell.addEventListener("click", (event) => event.stopPropagation());
      const wrap = UI.el("div", "inline");

      if (item.role === "payer") {
        if (item.requested) {
          wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareRequested")));
          wrap.appendChild(iconButton("check", "detail.approve", () => actOnShare(item, "settle")));
          wrap.appendChild(iconButton("x", "detail.reject", () => actOnShare(item, "reject")));
        } else {
          wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareOpen")));
          wrap.appendChild(iconButton("check", "detail.shareMarkSettled", () => actOnShare(item, "settle")));
        }
      } else if (item.requested) {
        wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.sharePending")));
      } else {
        wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareOpen")));
        wrap.appendChild(iconButton("check", "detail.iHavePaid", () => actOnShare(item, "request")));
      }

      Icons.apply(wrap);
      actionCell.appendChild(wrap);
      row.appendChild(actionCell);
      settleBody.appendChild(row);
    }
    Rows.apply(settleBody);
  }

  function renderAll() {
    renderSummary(cache.bills, cache.balances);
    renderBalances(cache.balances);
    renderSettlements(cache.settlements);
    renderBills(cache.bills);
  }

  Chrome.onLanguageChange = renderAll;

  Chrome.start({
    async onReady() {
      const [bills, balances, settlements] = await Promise.all([
        API.listBills(),
        API.balances(),
        API.settlements(),
      ]);
      cache.bills = bills;
      cache.balances = balances;
      cache.settlements = settlements;
      renderAll();
    },
  });
})();
