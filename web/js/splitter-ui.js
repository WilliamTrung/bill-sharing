(() => {
  const MAX_FILES = 10;
  const MAX_BYTES = 25 * 1024 * 1024;
  const ALLOWED = new Set(["image/jpeg", "image/png", "image/webp", "application/pdf"]);

  const state = {
    members: [],
    groups: [],
    selected: new Map(),
    mode: "EVEN",
    files: [],
    currentUserID: null,
  };

  const form = document.getElementById("bill-form");
  const amountInput = document.getElementById("b-amount");
  const dateInput = document.getElementById("b-date");
  const payerInput = document.getElementById("b-payer");
  const payerButton = document.getElementById("b-payer-button");
  const payerCurrent = document.getElementById("b-payer-current");
  const payerModal = document.getElementById("payer-modal");
  const payerSearch = document.getElementById("payer-search");
  const payerList = document.getElementById("payer-list");
  const payerEmpty = document.getElementById("payer-empty");
  const payerClose = document.getElementById("payer-close");
  const payerCancel = document.getElementById("payer-cancel");
  const participantsWrap = document.getElementById("participants");
  const participantsEmpty = document.getElementById("participants-empty");
  const participantsNoMatch = document.getElementById("participants-no-match");
  const participantSearch = document.getElementById("participant-search");
  const preview = document.getElementById("preview");
  const previewTotal = document.getElementById("preview-total");
  const previewTotalAmount = document.getElementById("preview-total-amount");
  const previewStatus = document.getElementById("preview-status");
  const previewWarning = document.getElementById("preview-warning");
  const billError = document.getElementById("bill-error");
  const submitBtn = document.getElementById("bill-submit");
  const fileInput = document.getElementById("file-input");
  const dropzone = document.getElementById("dropzone");
  const thumbs = document.getElementById("thumbs");
  const fileError = document.getElementById("file-error");

  function allocate(totalCents, weights) {
    const scaled = weights.map((weight) => {
      let value = BigInt(Math.round(Number(weight) * 1e6));
      if (value <= 0n) value = 1n;
      return value;
    });
    const weightSum = scaled.reduce((acc, value) => acc + value, 0n);
    const total = BigInt(totalCents);

    const shares = [];
    const remainders = [];
    let allocated = 0n;
    for (const value of scaled) {
      const numerator = total * value;
      const quotient = numerator / weightSum;
      shares.push(quotient);
      remainders.push(numerator % weightSum);
      allocated += quotient;
    }

    let leftover = Number(total - allocated);
    const order = shares.map((_, index) => index).sort((a, b) => {
      if (remainders[a] === remainders[b]) return a - b;
      return remainders[a] > remainders[b] ? -1 : 1;
    });
    for (let i = 0; i < leftover && i < order.length; i += 1) {
      shares[order[i]] += 1n;
    }
    return shares;
  }

  function activeMembers() {
    return state.members.filter((m) => m.is_active);
  }

  function activeGroups() {
    return state.groups.filter((g) => g.is_active);
  }

  function memberName(id) {
    const member = state.members.find((m) => m.id === id);
    return member ? member.name : `member #${id}`;
  }

  function groupMemberIds(group) {
    return group.members.map((m) => m.member_id);
  }

  function selectionKey(type, id) {
    return `${type}:${id}`;
  }

  function selectedEntries() {
    return Array.from(state.selected.entries()).map(([key, value]) => ({ key, ...value }));
  }

  function conflictFor(type, id) {
    for (const entry of state.selected.values()) {
      if (entry.type === "MEMBER" && type === "GROUP") {
        const group = activeGroups().find((g) => g.id === id);
        if (group && groupMemberIds(group).includes(entry.id)) {
          return I18n.t("bill.selectedIndividually", { name: memberName(entry.id) });
        }
      }
      if (entry.type === "GROUP" && type === "MEMBER") {
        const selectedGroup = activeGroups().find((g) => g.id === entry.id);
        if (selectedGroup && groupMemberIds(selectedGroup).includes(id)) {
          return I18n.t("bill.includedIn", { name: memberName(id), group: selectedGroup.name });
        }
      }
      if (entry.type === "GROUP" && type === "GROUP" && entry.id !== id) {
        const selectedGroup = activeGroups().find((g) => g.id === entry.id);
        const candidate = activeGroups().find((g) => g.id === id);
        if (selectedGroup && candidate) {
          const shared = groupMemberIds(candidate).filter((memberId) => groupMemberIds(selectedGroup).includes(memberId));
          if (shared.length > 0) {
            return I18n.t("bill.sharesWith", { names: shared.map(memberName).join(", "), group: selectedGroup.name });
          }
        }
      }
    }
    return null;
  }

  // Ratio weights live in the DOM; capture them before any re-render so typed
  // values survive filtering and further selections.
  function syncWeights() {
    if (state.mode !== "RATIO") return;
    for (const [key, entry] of state.selected) {
      const input = document.querySelector(`[data-key="${key}"] input[data-role=weight]`);
      if (!input) continue;
      const value = Number.parseFloat(input.value);
      if (Number.isFinite(value) && value > 0) entry.weight = value;
    }
  }

  function renderParticipants() {
    syncWeights();
    participantsWrap.replaceChildren();
    participantsNoMatch.classList.add("hidden");
    const actives = activeMembers();
    if (actives.length === 0) {
      participantsEmpty.classList.remove("hidden");
      return;
    }
    participantsEmpty.classList.add("hidden");

    const query = participantSearch.value.trim().toLowerCase();
    const matches = (name) => !query || name.toLowerCase().includes(query);

    // A selected entry always stays visible so it can be unchecked.
    let shown = 0;
    for (const member of actives) {
      if (!matches(member.name) && !state.selected.has(selectionKey("MEMBER", member.id))) continue;
      participantsWrap.appendChild(buildChoice("MEMBER", member));
      shown += 1;
    }
    for (const group of activeGroups()) {
      if (!matches(group.name) && !state.selected.has(selectionKey("GROUP", group.id))) continue;
      participantsWrap.appendChild(buildChoice("GROUP", group));
      shown += 1;
    }

    if (shown === 0) participantsNoMatch.classList.remove("hidden");
    refreshConflictState();
  }

  function buildChoice(type, entity) {
    const key = selectionKey(type, entity.id);
    const label = document.createElement("label");
    label.className = "choice";
    label.dataset.key = key;

    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    checkbox.checked = state.selected.has(key);
    checkbox.dataset.type = type;
    checkbox.dataset.id = String(entity.id);
    label.appendChild(checkbox);

    const body = UI.el("div", "choice-body");
    const title = UI.el("div", "choice-title");
    if (type === "MEMBER") {
      title.appendChild(UI.avatar(entity.name, entity.avatar_color, 22));
    } else {
      title.appendChild(UI.el("span", "pill pill-accent", I18n.t("bill.groupPill")));
    }
    title.appendChild(UI.el("span", "", entity.name));
    if (state.mode === "RATIO" && state.selected.has(key)) {
      title.appendChild(UI.el("span", "pill pill-warn", I18n.t("bill.weighted")));
    }
    body.appendChild(title);

    const sub =
      type === "MEMBER"
        ? entity.phone || entity.email || ""
        : I18n.t("bill.groupSummary", {
            count: entity.members.length,
            kind: I18n.t(entity.internal_split_type === "RATIO" ? "admin.internalRatio" : "admin.internalEven"),
          });
    if (sub) body.appendChild(UI.el("div", "choice-sub", sub));

    if (state.mode === "RATIO") {
      const ratioBox = UI.el("div", "ratio-box");
      ratioBox.appendChild(UI.el("span", "small faint", I18n.t("bill.weight")));
      const input = document.createElement("input");
      input.type = "number";
      input.min = "0.01";
      input.step = "0.01";
      input.value = state.selected.has(key) ? state.selected.get(key).weight : 1;
      input.dataset.role = "weight";
      input.setAttribute("aria-label", I18n.t("admin.ratioFor", { name: entity.name }));
      ratioBox.appendChild(input);
      body.appendChild(ratioBox);
    }

    const note = UI.el("div", "conflict-note hidden");
    body.appendChild(note);

    label.appendChild(body);

    checkbox.addEventListener("change", () => toggleSelection(type, entity));
    return label;
  }

  function toggleSelection(type, entity) {
    const key = selectionKey(type, entity.id);
    if (state.selected.has(key)) {
      state.selected.delete(key);
    } else {
      state.selected.set(key, { type, id: entity.id, weight: 1 });
    }
    renderParticipants();
    renderPreview();
  }

  function refreshConflictState() {
    document.querySelectorAll("#participants .choice").forEach((label) => {
      const checkbox = label.querySelector("input[type=checkbox]");
      const type = checkbox.dataset.type;
      const id = Number(checkbox.dataset.id);
      const key = selectionKey(type, id);
      const selected = state.selected.has(key);
      const conflict = selected ? null : conflictFor(type, id);
      const note = label.querySelector(".conflict-note");

      label.classList.toggle("disabled", Boolean(conflict));
      checkbox.disabled = Boolean(conflict);
      if (conflict) {
        note.textContent = I18n.t("bill.disabledPrefix", { reason: conflict });
        note.classList.remove("hidden");
      } else {
        note.textContent = "";
        note.classList.add("hidden");
      }
    });
  }

  function currentWeights(entries) {
    if (state.mode === "EVEN") return entries.map(() => 1);
    return entries.map((entry) => {
      const choice = document.querySelector(`[data-key="${entry.key}"]`);
      const input = choice ? choice.querySelector("input[data-role=weight]") : null;
      const value = input ? Number.parseFloat(input.value) : entry.weight;
      return Number.isFinite(value) && value > 0 ? value : 0;
    });
  }

  function computePlan() {
    const totalCents = Money.parse(amountInput.value);
    const entries = selectedEntries();
    if (entries.length === 0) return { entries, valid: false, reason: I18n.t("bill.errSelectOne") };
    if (totalCents <= 0) return { entries, valid: false, reason: I18n.t("bill.errTotal") };

    const weights = currentWeights(entries);
    if (weights.some((w) => w <= 0)) return { entries, valid: false, reason: I18n.t("bill.errWeightZero") };

    let top;
    try {
      top = allocate(totalCents, weights);
    } catch (err) {
      return { entries, valid: false, reason: I18n.t("bill.errSplitFail") };
    }

    const rows = [];
    for (let i = 0; i < entries.length; i += 1) {
      const entry = entries[i];
      if (entry.type === "MEMBER") {
        rows.push({
          kind: "MEMBER",
          name: memberName(entry.id),
          amount: top[i],
          weight: weights[i],
        });
        continue;
      }
      const group = activeGroups().find((g) => g.id === entry.id);
      if (!group) continue;
      const memberWeights =
        group.internal_split_type === "RATIO"
          ? group.members.map((m) => m.internal_ratio)
          : group.members.map(() => 1);
      const inner = allocate(top[i], memberWeights);
      rows.push({
        kind: "GROUP",
        name: group.name,
        amount: top[i],
        weight: weights[i],
        children: group.members.map((member, index) => ({
          name: member.name,
          amount: inner[index],
        })),
      });
    }

    return { entries, rows, totalCents, valid: true };
  }

  function renderPreview() {
    const plan = computePlan();
    preview.replaceChildren();

    if (!plan.valid) {
      preview.appendChild(UI.el("div", "empty", plan.reason));
      previewTotal.classList.add("hidden");
      previewStatus.textContent = I18n.t("bill.incomplete");
      previewStatus.className = "pill pill-warn";
      previewWarning.classList.add("hidden");
      return;
    }

    for (const row of plan.rows) {
      const line = UI.el("div", "preview-row");
      const labelCell = UI.el("div", "label-cell");
      if (row.kind === "GROUP") {
        labelCell.appendChild(UI.el("span", "pill pill-accent", I18n.t("bill.groupPill")));
      } else {
        labelCell.appendChild(UI.avatar(row.name, null, 22));
      }
      labelCell.appendChild(UI.el("span", "", row.name));
      line.appendChild(labelCell);

      const amountCell = UI.el("span", "amount", Money.format(row.amount));
      if (state.mode === "RATIO") {
        amountCell.title = I18n.t("bill.weightTitle", { weight: row.weight });
      }
      line.appendChild(amountCell);
      preview.appendChild(line);

      if (row.children) {
        for (const child of row.children) {
          const sub = UI.el("div", "preview-row sub");
          sub.appendChild(UI.el("span", "", child.name));
          sub.appendChild(UI.el("span", "amount", Money.format(child.amount)));
          preview.appendChild(sub);
        }
      }
    }

    let sum = 0n;
    for (const row of plan.rows) sum += row.amount;
    const mismatch = sum !== BigInt(plan.totalCents);

    previewTotal.classList.remove("hidden");
    previewTotalAmount.textContent = Money.format(sum);
    previewStatus.textContent = `${plan.rows.length} ${I18n.t(plan.rows.length === 1 ? "unit.participant" : "unit.participants")}`;
    previewStatus.className = "pill pill-accent";

    if (mismatch) {
      previewWarning.textContent = I18n.t("bill.allocMismatch");
      previewWarning.classList.remove("hidden");
    } else {
      previewWarning.classList.add("hidden");
    }
  }

  function updateCurrencyLabel() {
    const label = document.getElementById("currency-label");
    if (label) label.textContent = API.currency();
  }

  function payer(id) {
    return state.members.find((m) => m.id === id) || null;
  }

  function memberContact(member) {
    return member.email || member.phone || "";
  }

  function renderPayer() {
    payerCurrent.replaceChildren();
    const chosen = payer(payerID());
    if (chosen) {
      payerCurrent.appendChild(UI.avatar(chosen.name, chosen.avatar_color, 22));
      payerCurrent.appendChild(UI.el("span", "strong", chosen.name));
    } else {
      payerCurrent.appendChild(UI.el("span", "faint", I18n.t("bill.payerPlaceholder")));
    }
    payerButton.disabled = activeMembers().length === 0;
  }

  function payerID() {
    const value = Number(payerInput.value);
    return Number.isFinite(value) && value > 0 ? value : null;
  }

  function setPayer(id) {
    payerInput.value = id ? String(id) : "";
    renderPayer();
  }

  function matchesSearch(member, query) {
    if (!query) return true;
    return [member.name, member.email, member.username]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query));
  }

  function renderPayerOptions() {
    payerList.replaceChildren();
    const query = payerSearch.value.trim().toLowerCase();
    const matches = activeMembers().filter((member) => matchesSearch(member, query));

    payerEmpty.classList.toggle("hidden", matches.length > 0);
    for (const member of matches) {
      payerList.appendChild(buildPayerOption(member));
    }
  }

  function buildPayerOption(member) {
    const option = document.createElement("button");
    option.type = "button";
    option.className = "choice payer-option";
    option.setAttribute("role", "option");
    option.setAttribute("aria-selected", String(member.id === payerID()));
    option.dataset.payerId = String(member.id);

    const body = UI.el("div", "choice-body");
    const title = UI.el("div", "choice-title");
    title.appendChild(UI.avatar(member.name, member.avatar_color, 26));
    title.appendChild(UI.el("span", "", member.name));
    const contact = memberContact(member);
    if (contact) title.appendChild(UI.el("span", "contact", contact));
    if (state.currentUserID && member.id === state.currentUserID) {
      title.appendChild(UI.el("span", "pill pill-accent me-pill", I18n.t("bill.youPill")));
    } else if (member.id === payerID()) {
      title.appendChild(UI.el("span", "pill pill-accent me-pill", I18n.t("bill.payerSelected")));
    }
    body.appendChild(title);
    option.appendChild(body);

    option.addEventListener("click", () => {
      setPayer(member.id);
      closePayerPicker();
    });
    return option;
  }

  function openPayerPicker() {
    if (activeMembers().length === 0) return;
    payerSearch.value = "";
    renderPayerOptions();
    payerModal.classList.add("open");
    payerSearch.focus();
  }

  function closePayerPicker() {
    payerModal.classList.remove("open");
  }

  function addFiles(incoming) {
    fileError.classList.add("hidden");
    const errors = [];
    for (const file of incoming) {
      if (state.files.length >= MAX_FILES) {
        errors.push(I18n.t("bill.maxFiles", { count: MAX_FILES }));
        break;
      }
      if (!ALLOWED.has(file.type)) {
        errors.push(I18n.t("bill.unsupportedType", { name: file.name }));
        continue;
      }
      const duplicate = state.files.some((f) => f.name === file.name && f.size === file.size && f.lastModified === file.lastModified);
      if (duplicate) continue;
      state.files.push(file);
    }

    const totalSize = state.files.reduce((acc, file) => acc + file.size, 0);
    if (totalSize > MAX_BYTES) {
      while (state.files.length && state.files.reduce((acc, file) => acc + file.size, 0) > MAX_BYTES) {
        state.files.pop();
      }
      errors.push(I18n.t("bill.overLimit"));
    }

    if (errors.length) {
      fileError.textContent = errors.join(" ");
      fileError.classList.remove("hidden");
    }
    renderThumbs();
  }

  function removeFile(index) {
    state.files.splice(index, 1);
    renderThumbs();
  }

  function renderThumbs() {
    thumbs.replaceChildren();
    state.files.forEach((file, index) => {
      const tile = UI.el("div", "thumb");
      if (file.type.startsWith("image/")) {
        const img = document.createElement("img");
        img.src = URL.createObjectURL(file);
        img.alt = file.name;
        img.onload = () => URL.revokeObjectURL(img.src);
        tile.appendChild(img);
      } else {
        const block = UI.el("div", "thumb-file");
        block.appendChild(UI.el("div", "ext", "PDF"));
        block.appendChild(UI.el("div", "", file.name));
        tile.appendChild(block);
      }

      const remove = UI.el("button", "thumb-remove", "×");
      remove.type = "button";
      remove.setAttribute("aria-label", I18n.t("bill.removeFile", { name: file.name }));
      remove.addEventListener("click", () => removeFile(index));
      tile.appendChild(remove);

      tile.appendChild(UI.el("div", "thumb-name", file.name));
      thumbs.appendChild(tile);
    });
  }

  function validate() {
    billError.classList.add("hidden");
    const title = form.elements.title.value.trim();
    if (!title) return I18n.t("bill.errTitleRequired");
    if (Money.parse(amountInput.value) <= 0) return I18n.t("bill.errAmount");
    if (!dateInput.value) return I18n.t("bill.errDate");
    if (!payerID()) return I18n.t("bill.errPayer");
    if (state.selected.size === 0) return I18n.t("bill.errParticipants");
    const plan = computePlan();
    if (!plan.valid) return plan.reason;
    return null;
  }

  async function submit() {
    const problem = validate();
    if (problem) {
      billError.textContent = problem;
      billError.classList.remove("hidden");
      return;
    }

    const entries = selectedEntries();
    const weights = currentWeights(entries);
    const payload = {
      title: form.elements.title.value.trim(),
      description: form.elements.description.value.trim(),
      total_amount_cents: Money.parse(amountInput.value),
      paid_by_member_id: payerID(),
      split_mode: state.mode,
      bill_date: dateInput.value,
      participants: entries.map((entry, index) => ({
        type: entry.type,
        id: entry.id,
        weight: state.mode === "RATIO" ? weights[index] : 1,
      })),
    };

    submitBtn.disabled = true;
    submitBtn.textContent = I18n.t("bill.saving");
    try {
      const created = await API.createBill(payload, state.files);
      UI.toast(I18n.t("bill.saved"), "success");
      window.location.replace(`/bill-detail.html?id=${created.id}`);
    } catch (err) {
      billError.textContent = UI.errorMessage(err);
      billError.classList.remove("hidden");
      submitBtn.disabled = false;
      submitBtn.textContent = I18n.t("bill.saveBill");
    }
  }

  function setMode(mode) {
    if (state.mode === mode) return;
    state.mode = mode;
    document.querySelectorAll("#b-split button").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.mode === mode);
    });
    renderParticipants();
    renderPreview();
  }

  async function load() {
    const [members, groups, session] = await Promise.all([
      API.listMembers(),
      API.listGroups(),
      API.currentSession(),
    ]);
    state.members = members;
    state.groups = groups;
    state.currentUserID = session && session.user ? session.user.id : null;

    const own = payer(state.currentUserID);
    setPayer(own && own.is_active ? own.id : null);

    renderParticipants();
    renderPreview();
    updateCurrencyLabel();
  }

  payerButton.addEventListener("click", openPayerPicker);
  payerSearch.addEventListener("input", renderPayerOptions);
  payerClose.addEventListener("click", closePayerPicker);
  payerCancel.addEventListener("click", closePayerPicker);
  payerModal.addEventListener("click", (event) => {
    if (event.target === payerModal) closePayerPicker();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && payerModal.classList.contains("open")) closePayerPicker();
  });

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    submit();
  });

  document.querySelectorAll("#b-split button").forEach((btn) => {
    btn.addEventListener("click", () => setMode(btn.dataset.mode));
  });

  const refresh = UI.debounce(() => renderPreview(), 120);
  amountInput.addEventListener("input", () => {
    updateCurrencyLabel();
    refresh();
  });
  participantSearch.addEventListener("input", () => renderParticipants());
  participantsWrap.addEventListener("input", (event) => {
    if (event.target.matches("input[data-role=weight]")) renderPreview();
  });

  dropzone.addEventListener("click", () => fileInput.click());
  dropzone.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      fileInput.click();
    }
  });
  fileInput.addEventListener("change", () => {
    addFiles(Array.from(fileInput.files || []));
    fileInput.value = "";
  });

  ["dragenter", "dragover"].forEach((name) => {
    dropzone.addEventListener(name, (event) => {
      event.preventDefault();
      dropzone.classList.add("dragover");
    });
  });
  ["dragleave", "drop"].forEach((name) => {
    dropzone.addEventListener(name, (event) => {
      event.preventDefault();
      dropzone.classList.remove("dragover");
    });
  });
  dropzone.addEventListener("drop", (event) => {
    const dropped = event.dataTransfer && event.dataTransfer.files ? Array.from(event.dataTransfer.files) : [];
    if (dropped.length) addFiles(dropped);
  });

  dateInput.value = UI.todayISO();

  Chrome.onLanguageChange = () => {
    renderPayer();
    renderPayerOptions();
    renderParticipants();
    renderPreview();
    renderThumbs();
    updateCurrencyLabel();
  };

  Chrome.start({
    async onReady() {
      await load();
      updateCurrencyLabel();
    },
  });
})();
