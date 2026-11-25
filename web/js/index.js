(() => {
  const summary = document.getElementById("summary");
  const balancesBody = document.getElementById("balances-body");
  const balancesEmpty = document.getElementById("balances-empty");
  const billsBody = document.getElementById("bills-body");
  const billsEmpty = document.getElementById("bills-empty");

  const cache = { bills: [], balances: [] };

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
    if (balances.length === 0) {
      balancesEmpty.classList.remove("hidden");
      return;
    }
    balancesEmpty.classList.add("hidden");

    for (const balance of balances) {
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

      row.appendChild(UI.el("td", "right num muted", Money.format(balance.paid_cents)));
      row.appendChild(UI.el("td", "right num muted", Money.format(balance.owed_cents)));

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

      row.appendChild(UI.el("td", "right num strong", Money.format(bill.total_amount_cents)));

      billsBody.appendChild(row);
    }
  }

  function renderAll() {
    renderSummary(cache.bills, cache.balances);
    renderBalances(cache.balances);
    renderBills(cache.bills);
  }

  Chrome.onLanguageChange = renderAll;

  Chrome.start({
    async onReady() {
      const [bills, balances] = await Promise.all([API.listBills(), API.balances()]);
      cache.bills = bills;
      cache.balances = balances;
      renderAll();
    },
  });
})();
