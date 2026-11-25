(() => {
  const billId = Number(new URLSearchParams(window.location.search).get("id"));
  const cache = { bill: null };

  const titleEl = document.getElementById("bill-title");
  const summaryFacts = document.getElementById("summary-facts");
  const sharesBody = document.getElementById("shares-body");
  const sharesHint = document.getElementById("shares-hint");
  const sharesTotalAmount = document.getElementById("shares-total-amount");
  const participantsBody = document.getElementById("participants-body");
  const participantsEmpty = document.getElementById("participants-empty");
  const participantsHint = document.getElementById("participants-hint");
  const gallery = document.getElementById("gallery");
  const evidenceEmpty = document.getElementById("evidence-empty");
  const evidenceHint = document.getElementById("evidence-hint");
  const deleteBtn = document.getElementById("delete-bill");
  const toggleBtn = document.getElementById("toggle-status");
  const signInLink = document.getElementById("sign-in-link");
  const appNav = document.getElementById("app-nav");
  const signOutBtn = document.querySelector("[data-logout]");
  const lightbox = document.getElementById("lightbox");
  const lightboxContent = document.getElementById("lightbox-content");

  const state = { authenticated: false, userId: null, isAdmin: false };

  // Only the member who paid the bill may confirm shares; admins may too.
  function canSettleShares() {
    if (!state.authenticated || !cache.bill) return false;
    return state.isAdmin || cache.bill.paid_by_member_id === state.userId;
  }

  function factRow(label, value) {
    const row = UI.el("div", "spread");
    row.appendChild(UI.el("span", "small muted", label));
    const valueEl = UI.el("span", "strong", value);
    row.appendChild(valueEl);
    return row;
  }

  function statusPill(status) {
    const settled = status === "SETTLED";
    return UI.el(
      "span",
      `pill ${settled ? "pill-warn" : "pill-accent"}`,
      I18n.t(settled ? "public.statusSettled" : "public.statusOpen")
    );
  }

  function factPillRow(label, node) {
    const row = UI.el("div", "spread");
    row.appendChild(UI.el("span", "small muted", label));
    row.appendChild(node);
    return row;
  }

  function renderFacts() {
    const bill = cache.bill;
    titleEl.textContent = bill.title;

    summaryFacts.replaceChildren();
    if (bill.description) {
      const description = UI.el("div", "notice", bill.description);
      summaryFacts.appendChild(description);
    }
    summaryFacts.appendChild(factRow(I18n.t("detail.total"), Money.format(bill.total_amount_cents)));
    summaryFacts.appendChild(factPillRow(I18n.t("detail.status"), statusPill(bill.status)));
    summaryFacts.appendChild(factRow(I18n.t("detail.paidBy"), bill.paid_by_name));
    summaryFacts.appendChild(factRow(I18n.t("detail.date"), UI.formatDate(bill.bill_date)));
    summaryFacts.appendChild(
      factRow(I18n.t("detail.split"), I18n.t(bill.split_mode === "RATIO" ? "index.splitRatio" : "index.splitEven"))
    );
    if (bill.created_at) {
      summaryFacts.appendChild(factRow(I18n.t("detail.created"), UI.formatDate(bill.created_at)));
    }
  }

  function renderActions() {
    const settled = cache.bill && cache.bill.status === "SETTLED";

    appNav.classList.toggle("hidden", !state.authenticated);
    signInLink.classList.toggle("hidden", state.authenticated);
    if (signOutBtn) signOutBtn.classList.toggle("hidden", !state.authenticated);

    toggleBtn.classList.toggle("hidden", !state.authenticated);
    deleteBtn.classList.toggle("hidden", !state.authenticated);
    // Update the label span only, so the icon inserted by Icons.apply survives.
    const toggleLabel = toggleBtn.querySelector("[data-label]");
    const toggleText = I18n.t(settled ? "detail.markOpen" : "detail.markSettled");
    if (toggleLabel) toggleLabel.textContent = toggleText;
    else toggleBtn.textContent = toggleText;
  }

  function renderParticipants() {
    const bill = cache.bill;
    participantsBody.replaceChildren();

    if (!bill.participants || bill.participants.length === 0) {
      participantsEmpty.classList.remove("hidden");
      participantsHint.textContent = "";
      return;
    }
    participantsEmpty.classList.add("hidden");
    participantsHint.textContent = `${bill.participants.length} ${I18n.t(
      bill.participants.length === 1 ? "unit.participant" : "unit.participants"
    )}`;

    for (const participant of bill.participants) {
      const row = document.createElement("tr");
      const nameCell = document.createElement("td");
      const line = UI.el("div", "person");
      if (participant.participant_type === "GROUP") {
        line.appendChild(UI.el("span", "pill pill-accent", I18n.t("bill.groupPill")));
      } else {
        line.appendChild(UI.avatar(participant.name, null));
      }
      const who = UI.el("div", "who");
      who.appendChild(UI.el("div", "name", participant.name));
      if (participant.note) who.appendChild(UI.el("div", "sub", participant.note));
      line.appendChild(who);
      nameCell.appendChild(line);
      row.appendChild(nameCell);

      row.appendChild(
        UI.el(
          "td",
          "small",
          bill.split_mode === "RATIO"
            ? `${I18n.t("detail.ratioMode")} · ${participant.ratio_weight}`
            : I18n.t("detail.evenMode")
        )
      );
      row.appendChild(UI.el("td", "right num strong", Money.format(participant.allocated_amount_cents)));
      participantsBody.appendChild(row);
    }
  }

  function renderShares() {
    const bill = cache.bill;
    sharesBody.replaceChildren();

    const shares = bill.member_shares || [];
    const groupNames = new Map();
    for (const participant of bill.participants || []) {
      if (participant.participant_type === "GROUP" && participant.group_id) {
        groupNames.set(participant.group_id, participant.name);
      }
    }

    // Participant notes belong to top-level participants, index them by member id
    // (falling back to the group id for group rows).
    const shareNotes = new Map();
    for (const participant of bill.participants || []) {
      if (!participant.note) continue;
      if (participant.member_id) shareNotes.set(`m${participant.member_id}`, participant.note);
      if (participant.group_id) shareNotes.set(`g${participant.group_id}`, participant.note);
    }

    let total = 0n;
    for (const share of shares) {
      const row = document.createElement("tr");

      const memberCell = document.createElement("td");
      const person = UI.el("div", "person");
      person.appendChild(UI.avatar(share.member_name, null));
      const who = UI.el("div", "who");
      who.appendChild(UI.el("div", "name", share.member_name));
      const shareNote = share.source_group_id
        ? shareNotes.get(`g${share.source_group_id}`)
        : shareNotes.get(`m${share.member_id}`);
      if (shareNote) who.appendChild(UI.el("div", "sub", shareNote));
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
      total += BigInt(share.owed_amount_cents);

      // Settlement cell. The payer sees approve/reject icons; a debtor sees a
      // "request" toggle (their claim still needs the payer's confirmation).
      const actionCell = document.createElement("td");
      actionCell.className = "right";
      const isPayerShare = cache.bill && share.member_id === cache.bill.paid_by_member_id;
      const settled = share.is_settled === true;
      const requested = Boolean(share.settle_requested_at);

      if (isPayerShare) {
        actionCell.appendChild(UI.el("span", "pill pill-accent", I18n.t("detail.payerPaid")));
      } else if (canSettleShares()) {
        const wrap = UI.el("div", "inline");
        if (settled) {
          wrap.appendChild(UI.el("span", "pill pill-accent", I18n.t("detail.shareSettled")));
          wrap.appendChild(settleIconButton("undo", "detail.shareMarkOpen", () => shareAction(share, "reopen")));
        } else if (requested) {
          wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareRequested")));
          wrap.appendChild(settleIconButton("check", "detail.approve", () => shareAction(share, "settle")));
          wrap.appendChild(settleIconButton("x", "detail.reject", () => shareAction(share, "reject")));
        } else {
          wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareOpen")));
          wrap.appendChild(settleIconButton("check", "detail.shareMarkSettled", () => shareAction(share, "settle")));
        }
        Icons.apply(wrap);
        actionCell.appendChild(wrap);
      } else if (isDebtorShare(share)) {
        const wrap = UI.el("div", "inline");
        if (settled) {
          wrap.appendChild(UI.el("span", "pill pill-accent", I18n.t("detail.shareSettled")));
        } else if (requested) {
          wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.sharePending")));
        } else {
          wrap.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareOpen")));
          wrap.appendChild(settleIconButton("check", "detail.iHavePaid", () => shareAction(share, "request")));
        }
        Icons.apply(wrap);
        actionCell.appendChild(wrap);
      } else if (settled) {
        actionCell.appendChild(UI.el("span", "pill pill-accent", I18n.t("detail.shareSettled")));
      } else if (requested) {
        actionCell.appendChild(UI.el("span", "pill pill-warn", I18n.t("detail.shareRequested")));
      }
      row.appendChild(actionCell);

      sharesBody.appendChild(row);
    }

    sharesTotalAmount.textContent = Money.format(total);
    sharesHint.textContent = `${shares.length} ${I18n.t(shares.length === 1 ? "unit.member" : "unit.members")}`;
  }

  function renderGallery() {
    const attachments = cache.bill.attachments || [];
    gallery.replaceChildren();
    evidenceHint.textContent = `${attachments.length} ${I18n.t(attachments.length === 1 ? "unit.attachment" : "unit.attachments")}`;

    if (attachments.length === 0) {
      evidenceEmpty.classList.remove("hidden");
      return;
    }
    evidenceEmpty.classList.add("hidden");

    for (const attachment of attachments) {
      const link = document.createElement("a");
      link.href = attachment.url;
      link.target = "_blank";
      link.rel = "noopener";
      link.setAttribute("aria-label", I18n.t("detail.viewFile", { name: attachment.original_filename }));

      if (attachment.mime_type.startsWith("image/")) {
        const img = document.createElement("img");
        img.src = attachment.url;
        img.alt = attachment.original_filename;
        img.loading = "lazy";
        link.appendChild(img);
        link.addEventListener("click", (event) => {
          event.preventDefault();
          openLightbox(attachment.url, "image");
        });
      } else {
        const tile = UI.el("div", "file-tile");
        tile.appendChild(UI.el("div", "strong", "PDF"));
        tile.appendChild(UI.el("div", "", attachment.original_filename));
        link.appendChild(tile);
        link.addEventListener("click", (event) => {
          event.preventDefault();
          openLightbox(attachment.url, "pdf");
        });
      }

      gallery.appendChild(link);
    }
  }

  function openLightbox(url, kind) {
    lightboxContent.replaceChildren();
    if (kind === "image") {
      const img = document.createElement("img");
      img.src = url;
      img.alt = "";
      lightboxContent.appendChild(img);
    } else {
      const frame = document.createElement("iframe");
      frame.src = url;
      frame.title = "PDF";
      lightboxContent.appendChild(frame);
    }
    lightbox.classList.add("open");
  }

  function closeLightbox() {
    lightbox.classList.remove("open");
    lightboxContent.replaceChildren();
  }

  function renderAll() {
    if (!cache.bill) return;
    renderFacts();
    renderActions();
    renderParticipants();
    renderShares();
    renderGallery();
  }

  // isDebtorShare reports whether the signed-in user owes this particular share.
  function isDebtorShare(share) {
    return Boolean(state.userId) && share.member_id === state.userId;
  }

  // settleIconButton builds an icon-only control with an accessible tooltip.
  function settleIconButton(icon, labelKey, onClick) {
    const label = I18n.t(labelKey);
    const button = UI.el("button", "btn btn-quiet btn-sm btn-icon");
    button.type = "button";
    button.dataset.icon = icon;
    button.title = label;
    button.setAttribute("aria-label", label);
    button.addEventListener("click", onClick);
    return button;
  }

  async function shareAction(share, action) {
    if (action === "settle") {
      const confirmed = window.confirm(
        I18n.t("detail.confirmSettle", { name: share.member_name, amount: Money.format(share.owed_amount_cents) })
      );
      if (!confirmed) return;
    }
    try {
      const result = await API.updateShareSettled(cache.bill.id, share.id, action);
      share.is_settled = result.is_settled;
      share.settle_requested_at = result.requested ? new Date().toISOString() : null;
      // The bill completes once every share is settled, so take the new status.
      if (result.bill_status) cache.bill.status = result.bill_status;
      renderAll();
      UI.toast(I18n.t("detail.shareUpdated"), "success");
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
    }
  }

  async function toggleStatus() {
    const bill = cache.bill;
    const next = bill.status === "SETTLED" ? "OPEN" : "SETTLED";
    toggleBtn.disabled = true;
    try {
      const result = await API.updateBillStatus(bill.id, next);
      bill.status = result.status;
      UI.toast(I18n.t("detail.statusUpdated"), "success");
      renderAll();
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
    } finally {
      toggleBtn.disabled = false;
    }
  }

  async function removeBill() {
    const bill = cache.bill;
    const confirmed = window.confirm(I18n.t("detail.confirmDelete", { title: bill.title }));
    if (!confirmed) return;

    deleteBtn.disabled = true;
    try {
      await API.deleteBill(bill.id);
      UI.toast(I18n.t("detail.deleted"), "success");
      window.location.replace("/dashboard.html");
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
      deleteBtn.disabled = false;
    }
  }

  deleteBtn.addEventListener("click", removeBill);
  toggleBtn.addEventListener("click", toggleStatus);
  document.getElementById("lightbox-close").addEventListener("click", closeLightbox);
  lightbox.addEventListener("click", (event) => {
    if (event.target === lightbox) closeLightbox();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && lightbox.classList.contains("open")) closeLightbox();
  });

  Chrome.onLanguageChange = renderAll;

  Chrome.start({
    async onReady() {
      if (!billId) {
        UI.toast(I18n.t("detail.notFound"), "error");
        window.location.replace("/");
        return;
      }
      const session = await API.currentSession();
      state.authenticated = session !== null;
      state.userId = session && session.user ? session.user.id : null;
      state.isAdmin = Boolean(session && session.is_admin);
      try {
        cache.bill = await API.publicBill(billId);
      } catch (err) {
        if (err && err.status === 404) {
          UI.toast(I18n.t("detail.notFound"), "error");
          window.location.replace("/");
          return;
        }
        throw err;
      }
      renderAll();
    },
  });
})();
