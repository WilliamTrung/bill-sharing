(() => {
  const summary = document.getElementById("summary");
  const billsBody = document.getElementById("bills-body");
  const billsEmpty = document.getElementById("bills-empty");
  const billsEmptyText = document.getElementById("bills-empty-text");
  const billsHint = document.getElementById("bills-hint");
  const filter = document.getElementById("bills-filter");

  const cache = { bills: [] };
  const state = { filter: "ongoing" };

  function unit(key, singularKey, count) {
    return I18n.t(count === 1 ? singularKey : key);
  }

  function statusPill(status) {
    const settled = status === "SETTLED";
    return UI.el(
      "span",
      `pill ${settled ? "pill-warn" : "pill-accent"}`,
      I18n.t(settled ? "public.statusSettled" : "public.statusOpen")
    );
  }

  function stat(label, value, meta) {
    const card = UI.el("div", "stat");
    card.appendChild(UI.el("div", "label", label));
    card.appendChild(UI.el("div", "value", value));
    if (meta) card.appendChild(UI.el("div", "meta", meta));
    return card;
  }

  function visibleBills() {
    if (state.filter === "all") return cache.bills;
    return cache.bills.filter((bill) => bill.status !== "SETTLED");
  }

  function renderSummary() {
    summary.replaceChildren();

    const ongoing = cache.bills.filter((bill) => bill.status !== "SETTLED");
    let total = 0n;
    for (const bill of ongoing) total += BigInt(bill.total_amount_cents || 0);

    const shared = new Set();
    for (const bill of cache.bills) {
      for (const name of bill.members || []) shared.add(name);
    }

    summary.appendChild(
      stat(
        I18n.t("public.ongoingCount"),
        String(ongoing.length),
        `${cache.bills.length} ${unit("unit.bills", "unit.bill", cache.bills.length)}`
      )
    );
    summary.appendChild(stat(I18n.t("public.totalTracked"), Money.format(total), I18n.t("public.ongoing")));
    summary.appendChild(
      stat(
        I18n.t("public.membersCount"),
        String(shared.size),
        `${shared.size} ${unit("unit.members", "unit.member", shared.size)}`
      )
    );
  }

  function renderBills() {
    billsBody.replaceChildren();
    const bills = visibleBills();

    if (bills.length === 0) {
      billsEmpty.classList.remove("hidden");
      billsEmptyText.textContent = I18n.t(cache.bills.length === 0 ? "public.none" : "public.noOngoing");
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
      // Stop the row click from navigating when the shares toggle is used.
      toggleCell.addEventListener("click", (event) => event.stopPropagation());
      row.appendChild(toggleCell);

      const titleCell = document.createElement("td");
      const link = document.createElement("a");
      link.href = `/bill-detail.html?id=${bill.id}`;
      link.className = "strong";
      link.textContent = bill.title;
      titleCell.appendChild(link);
      if (bill.description) titleCell.appendChild(UI.el("div", "small faint", bill.description));
      row.appendChild(titleCell);

      row.appendChild(UI.el("td", "small", UI.formatDate(bill.bill_date)));
      row.appendChild(UI.el("td", "small", bill.paid_by_name));

      const membersCell = document.createElement("td");
      const members = bill.members || [];
      if (members.length === 0) {
        membersCell.appendChild(UI.el("span", "faint small", "—"));
      } else {
        const chips = UI.el("div", "inline");
        for (const name of members) {
          chips.appendChild(UI.el("span", "pill", name));
        }
        membersCell.appendChild(chips);
      }
      row.appendChild(membersCell);

      const statusCell = document.createElement("td");
      statusCell.appendChild(statusPill(bill.status));
      row.appendChild(statusCell);

      row.appendChild(UI.el("td", "right num strong", Money.format(bill.total_amount_cents)));

      billsBody.appendChild(row);
      Shares.mount(toggleCell, row, 7, bill.id, API.publicBill);
    }
    Rows.apply(billsBody);
  }

  function renderFilters() {
    filter.querySelectorAll("button").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.filter === state.filter);
    });
  }

  function renderHint() {
    const shown = visibleBills().length;
    billsHint.textContent = `${shown} ${unit("unit.bills", "unit.bill", shown)}`;
  }

  function renderAll() {
    renderSummary();
    renderFilters();
    renderBills();
    renderHint();
  }

  filter.addEventListener("click", (event) => {
    const btn = event.target.closest("button[data-filter]");
    if (!btn) return;
    state.filter = btn.dataset.filter;
    renderAll();
  });

  Chrome.onLanguageChange = renderAll;

  Chrome.start({
    async onReady() {
      cache.bills = await API.publicBills();
      renderAll();
    },
  });
})();
