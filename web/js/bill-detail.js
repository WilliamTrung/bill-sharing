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
  const gallery = document.getElementById("gallery");
  const evidenceEmpty = document.getElementById("evidence-empty");
  const evidenceHint = document.getElementById("evidence-hint");
  const deleteBtn = document.getElementById("delete-bill");
  const lightbox = document.getElementById("lightbox");
  const lightboxContent = document.getElementById("lightbox-content");

  function factRow(label, value) {
    const row = UI.el("div", "spread");
    row.appendChild(UI.el("span", "small muted", label));
    const valueEl = UI.el("span", "strong", value);
    row.appendChild(valueEl);
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
    summaryFacts.appendChild(factRow(I18n.t("detail.paidBy"), bill.paid_by_name));
    summaryFacts.appendChild(factRow(I18n.t("detail.date"), UI.formatDate(bill.bill_date)));
    summaryFacts.appendChild(
      factRow(I18n.t("detail.split"), I18n.t(bill.split_mode === "RATIO" ? "index.splitRatio" : "index.splitEven"))
    );
    if (bill.created_at) {
      summaryFacts.appendChild(factRow(I18n.t("detail.created"), UI.formatDate(bill.created_at)));
    }
  }

  function renderParticipants() {
    const bill = cache.bill;
    participantsBody.replaceChildren();

    if (!bill.participants || bill.participants.length === 0) {
      participantsEmpty.classList.remove("hidden");
      return;
    }
    participantsEmpty.classList.add("hidden");

    for (const participant of bill.participants) {
      const row = document.createElement("tr");
      const nameCell = document.createElement("td");
      const line = UI.el("div", "person");
      if (participant.participant_type === "GROUP") {
        line.appendChild(UI.el("span", "pill pill-accent", I18n.t("bill.groupPill")));
      } else {
        line.appendChild(UI.avatar(participant.name, null));
      }
      line.appendChild(UI.el("span", "strong", participant.name));
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
    renderParticipants();
    renderShares();
    renderGallery();
  }

  async function removeBill() {
    const bill = cache.bill;
    const confirmed = window.confirm(I18n.t("detail.confirmDelete", { title: bill.title }));
    if (!confirmed) return;

    deleteBtn.disabled = true;
    try {
      await API.deleteBill(bill.id);
      UI.toast(I18n.t("detail.deleted"), "success");
      window.location.replace("/");
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
      deleteBtn.disabled = false;
    }
  }

  deleteBtn.addEventListener("click", removeBill);
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
      cache.bill = await API.getBill(billId);
      renderAll();
    },
  });
})();
