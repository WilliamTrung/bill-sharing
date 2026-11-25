(() => {
  const state = {
    members: [],
    groups: [],
    showInactive: true,
    editingMemberId: null,
    drawerMemberId: null,
    editingGroupId: null,
    groupSplitType: "EVEN",
    groupContextMemberId: null,
  };

  const membersBody = document.getElementById("members-body");
  const membersEmpty = document.getElementById("members-empty");
  const groupsGrid = document.getElementById("groups-grid");
  const groupsEmpty = document.getElementById("groups-empty");
  const showInactive = document.getElementById("show-inactive");

  const memberModal = document.getElementById("member-modal");
  const memberForm = document.getElementById("member-form");
  const memberFormError = document.getElementById("member-form-error");
  const memberModalTitle = document.getElementById("member-modal-title");

  const groupModal = document.getElementById("group-modal");
  const groupMembers = document.getElementById("g-members");
  const groupFormError = document.getElementById("group-form-error");
  const groupModalTitle = document.getElementById("group-modal-title");

  const backdrop = document.getElementById("backdrop");
  const drawer = document.getElementById("member-drawer");
  const drawerTitle = document.getElementById("drawer-title");
  const drawerBody = document.getElementById("drawer-body");
  const resendInviteBtn = document.getElementById("drawer-resend-invite");

  function memberById(id) {
    return state.members.find((m) => m.id === id);
  }

  function canInvite(member) {
    return member.has_password === false && Boolean(member.email) && !member.email.endsWith("@users.invalid");
  }

  // sendInvite dispatches an activation link and reports the result on the button.
  function sendInvite(member, button) {
    button.disabled = true;
    const label = button.textContent;
    button.textContent = I18n.t("admin.inviteSending");
    API.sendInvite(member.id)
      .then(() => UI.toast(I18n.t("admin.inviteSent"), "success"))
      .catch((err) => UI.toast(UI.errorMessage(err), "error"))
      .finally(() => {
        button.disabled = false;
        button.textContent = label;
      });
  }

  function renderMembers() {
    membersBody.replaceChildren();
    const visible = state.members.filter((m) => state.showInactive || m.is_active);

    if (visible.length === 0) {
      membersEmpty.classList.remove("hidden");
      return;
    }
    membersEmpty.classList.add("hidden");

    for (const member of visible) {
      const row = document.createElement("tr");
      if (!member.is_active) row.style.opacity = "0.6";

      const nameCell = document.createElement("td");
      const person = UI.el("div", "person");
      person.appendChild(UI.avatar(member.name, member.avatar_color));
      const who = UI.el("div", "who");
      who.appendChild(UI.el("div", "name", member.name));
      if (!member.is_active) who.appendChild(UI.el("div", "sub", I18n.t("state.inactive")));
      person.appendChild(who);
      nameCell.appendChild(person);
      row.appendChild(nameCell);

      const contact = document.createElement("td");
      contact.className = "small muted";
      contact.appendChild(UI.el("div", "", member.phone || "—"));
      if (member.email) contact.appendChild(UI.el("div", "faint", member.email));
      row.appendChild(contact);

      const groupsCell = document.createElement("td");
      const groupWrap = UI.el("div", "inline");
      if (member.groups && member.groups.length) {
        for (const group of member.groups) {
          groupWrap.appendChild(UI.el("span", "pill pill-accent", group.name));
        }
      } else {
        groupWrap.appendChild(UI.el("span", "faint small", I18n.t("admin.none")));
      }
      groupsCell.appendChild(groupWrap);
      row.appendChild(groupsCell);

      const actions = document.createElement("td");
      const actionsWrap = UI.el("div", "member-row-actions");
      const detailBtn = UI.el("button", "btn btn-quiet btn-sm", I18n.t("action.details"));
      detailBtn.type = "button";
      detailBtn.addEventListener("click", () => openDrawer(member.id));
      actionsWrap.appendChild(detailBtn);
      const editBtn = UI.el("button", "btn btn-quiet btn-sm", I18n.t("action.edit"));
      editBtn.type = "button";
      editBtn.addEventListener("click", () => openMemberModal(member));
      actionsWrap.appendChild(editBtn);

      if (canInvite(member)) {
        const inviteBtn = UI.el("button", "btn btn-ghost btn-sm", I18n.t("admin.resendInvite"));
        inviteBtn.type = "button";
        inviteBtn.title = I18n.t("admin.resendInviteHint");
        inviteBtn.addEventListener("click", () => sendInvite(member, inviteBtn));
        actionsWrap.appendChild(inviteBtn);
      }

      actions.appendChild(actionsWrap);
      row.appendChild(actions);

      membersBody.appendChild(row);
    }
  }

  function renderGroups() {
    groupsGrid.replaceChildren();
    if (state.groups.length === 0) {
      groupsEmpty.classList.remove("hidden");
      return;
    }
    groupsEmpty.classList.add("hidden");

    for (const group of state.groups) {
      const card = UI.el("div", "group-card");
      if (!group.is_active) card.style.opacity = "0.65";

      const head = UI.el("div", "spread");
      const titleWrap = UI.el("div", "inline");
      titleWrap.appendChild(UI.el("strong", "", group.name));
      titleWrap.appendChild(
        UI.el(
          "span",
          `pill ${group.internal_split_type === "RATIO" ? "pill-warn" : "pill-accent"}`,
          I18n.t(group.internal_split_type === "RATIO" ? "admin.splitRatio" : "admin.splitEven")
        )
      );
      if (!group.is_active) titleWrap.appendChild(UI.el("span", "pill", I18n.t("state.inactive")));
      head.appendChild(titleWrap);

      const editBtn = UI.el("button", "btn btn-quiet btn-sm", I18n.t("action.edit"));
      editBtn.type = "button";
      editBtn.addEventListener("click", () => openGroupModal(group));
      head.appendChild(editBtn);
      card.appendChild(head);

      const membersWrap = UI.el("div", "members");
      for (const member of group.members) {
        const label =
          group.internal_split_type === "RATIO"
            ? `${member.name} (${formatRatio(member.internal_ratio)})`
            : member.name;
        membersWrap.appendChild(UI.el("span", "pill", label));
      }
      card.appendChild(membersWrap);
      groupsGrid.appendChild(card);
    }
  }

  function formatRatio(value) {
    const number = Number(value);
    if (!Number.isFinite(number)) return "1";
    return Number.isInteger(number) ? String(number) : number.toFixed(2).replace(/\.?0+$/, "");
  }

  function openMemberModal(member) {
    state.editingMemberId = member ? member.id : null;
    memberModalTitle.textContent = I18n.t(member ? "admin.editMemberTitle" : "admin.addMemberTitle");
    memberFormError.classList.add("hidden");

    memberForm.elements.name.value = member ? member.name : "";
    memberForm.elements.phone.value = member ? member.phone || "" : "";
    memberForm.elements.email.value = member ? member.email || "" : "";
    memberForm.elements.notes.value = member ? member.notes || "" : "";
    document.getElementById("m-active").checked = member ? member.is_active : true;

    memberModal.classList.add("open");
    memberForm.elements.name.focus();
  }

  function closeMemberModal() {
    memberModal.classList.remove("open");
    state.editingMemberId = null;
  }

  async function saveMember() {
    memberFormError.classList.add("hidden");
    const payload = {
      name: memberForm.elements.name.value.trim(),
      phone: memberForm.elements.phone.value.trim(),
      email: memberForm.elements.email.value.trim(),
      notes: memberForm.elements.notes.value.trim(),
      is_active: document.getElementById("m-active").checked,
    };
    if (!payload.name) {
      memberFormError.textContent = I18n.t("admin.errNameRequired");
      memberFormError.classList.remove("hidden");
      return;
    }

    const saveBtn = document.getElementById("member-save");
    saveBtn.disabled = true;
    try {
      if (state.editingMemberId) {
        await API.updateMember(state.editingMemberId, payload);
        UI.toast(I18n.t("admin.memberUpdated"), "success");
      } else {
        await API.createMember(payload);
        UI.toast(I18n.t("admin.memberAdded"), "success");
      }
      closeMemberModal();
      await loadAll();
    } catch (err) {
      memberFormError.textContent = UI.errorMessage(err);
      memberFormError.classList.remove("hidden");
    } finally {
      saveBtn.disabled = false;
    }
  }

  function openGroupModal(group) {
    state.editingGroupId = group ? group.id : null;
    groupModalTitle.textContent = I18n.t(group ? "admin.editGroupTitle" : "admin.newGroupTitle");
    groupFormError.classList.add("hidden");

    document.getElementById("g-name").value = group ? group.name : "";
    document.getElementById("g-active").checked = group ? group.is_active : true;
    setGroupSplit(group ? group.internal_split_type : "EVEN");

    const preselected = new Map();
    if (group) {
      for (const member of group.members) {
        preselected.set(member.member_id, member.internal_ratio);
      }
    } else if (state.groupContextMemberId) {
      preselected.set(state.groupContextMemberId, 1);
    }

    renderGroupMemberChoices(preselected);
    groupModal.classList.add("open");
    document.getElementById("g-name").focus();
  }

  function closeGroupModal() {
    groupModal.classList.remove("open");
    state.groupContextMemberId = null;
    state.editingGroupId = null;
  }

  function setGroupSplit(type) {
    state.groupSplitType = type;
    document.querySelectorAll("#g-split button").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.split === type);
    });
    document.querySelectorAll("#g-members .ratio-box").forEach((box) => {
      box.classList.toggle("hidden", type !== "RATIO");
    });
  }

  function renderGroupMemberChoices(preselected) {
    groupMembers.replaceChildren();
    const actives = state.members.filter((m) => m.is_active);

    if (actives.length < 2) {
      groupMembers.appendChild(UI.el("div", "notice warn", I18n.t("admin.addTwoActives")));
      return;
    }

    for (const member of actives) {
      const label = document.createElement("label");
      label.className = "choice";

      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.value = String(member.id);
      checkbox.checked = preselected.has(member.id);
      checkbox.dataset.memberId = String(member.id);

      const body = UI.el("div", "choice-body");
      const title = UI.el("div", "choice-title");
      title.appendChild(UI.avatar(member.name, member.avatar_color, 22));
      title.appendChild(UI.el("span", "", member.name));
      body.appendChild(title);
      if (member.phone) body.appendChild(UI.el("div", "choice-sub", member.phone));

      const ratioBox = UI.el("div", "ratio-box");
      const ratioInput = document.createElement("input");
      ratioInput.type = "number";
      ratioInput.min = "0.01";
      ratioInput.step = "0.01";
      ratioInput.value = formatRatio(preselected.get(member.id) || 1);
      ratioInput.setAttribute("aria-label", I18n.t("admin.ratioFor", { name: member.name }));
      ratioBox.appendChild(UI.el("span", "small faint", I18n.t("admin.ratio")));
      ratioBox.appendChild(ratioInput);
      body.appendChild(ratioBox);

      label.appendChild(checkbox);
      label.appendChild(body);
      groupMembers.appendChild(label);
    }
    setGroupSplit(state.groupSplitType);
  }

  async function saveGroup() {
    groupFormError.classList.add("hidden");
    const name = document.getElementById("g-name").value.trim();
    if (!name) {
      groupFormError.textContent = I18n.t("admin.errGroupNameRequired");
      groupFormError.classList.remove("hidden");
      return;
    }

    const members = [];
    let invalidRatio = null;
    document.querySelectorAll("#g-members input[type=checkbox]:checked").forEach((checkbox) => {
      const ratioInput = checkbox.closest(".choice").querySelector("input[type=number]");
      const ratio = state.groupSplitType === "RATIO" ? Number.parseFloat(ratioInput.value) : 1;
      if (state.groupSplitType === "RATIO" && (!Number.isFinite(ratio) || ratio <= 0)) {
        invalidRatio = checkbox.closest(".choice").querySelector(".choice-title span").textContent;
      }
      members.push({ member_id: Number(checkbox.dataset.memberId), internal_ratio: ratio });
    });

    if (invalidRatio) {
      groupFormError.textContent = I18n.t("admin.errRatio", { name: invalidRatio });
      groupFormError.classList.remove("hidden");
      return;
    }
    if (members.length < 2) {
      groupFormError.textContent = I18n.t("admin.errPickTwo");
      groupFormError.classList.remove("hidden");
      return;
    }

    const payload = {
      name,
      internal_split_type: state.groupSplitType,
      is_active: document.getElementById("g-active").checked,
      members,
    };

    const saveBtn = document.getElementById("group-save");
    saveBtn.disabled = true;
    try {
      if (state.editingGroupId) {
        await API.updateGroup(state.editingGroupId, payload);
        UI.toast(I18n.t("admin.groupUpdated"), "success");
      } else {
        await API.createGroup(payload);
        UI.toast(I18n.t("admin.groupCreated"), "success");
      }
      closeGroupModal();
      await loadAll();
    } catch (err) {
      groupFormError.textContent = UI.errorMessage(err);
      groupFormError.classList.remove("hidden");
    } finally {
      saveBtn.disabled = false;
    }
  }

  function closeDrawer() {
    drawer.classList.remove("open");
    backdrop.classList.remove("open");
    state.drawerMemberId = null;
  }

  function openDrawer(memberId) {
    const member = memberById(memberId);
    if (!member) return;
    state.drawerMemberId = memberId;
    drawerTitle.textContent = member.name;
    drawerBody.replaceChildren();

    const person = UI.el("div", "person");
    person.appendChild(UI.avatar(member.name, member.avatar_color, 44));
    const who = UI.el("div", "who");
    who.appendChild(UI.el("div", "strong", member.name));
    who.appendChild(UI.el("div", "sub", member.phone || member.email || I18n.t(member.is_active ? "state.active" : "state.inactive")));
    person.appendChild(who);
    drawerBody.appendChild(person);

    drawerBody.appendChild(UI.el("div", "divider"));

    const facts = UI.el("div", "stack");
    addFact(facts, I18n.t("admin.phone"), member.phone || "—");
    addFact(facts, I18n.t("admin.email"), member.email || "—");
    addFact(facts, I18n.t("admin.status"), I18n.t(member.is_active ? "state.active" : "state.inactive"));
    addFact(facts, I18n.t("admin.notes"), member.notes || "—");
    drawerBody.appendChild(facts);

    drawerBody.appendChild(UI.el("div", "divider"));

    drawerBody.appendChild(UI.el("h3", "", I18n.t("admin.groups")));
    const groupsWrap = UI.el("div", "inline");
    if (member.groups && member.groups.length) {
      for (const group of member.groups) {
        groupsWrap.appendChild(UI.el("span", "pill pill-accent", group.name));
      }
    } else {
      groupsWrap.appendChild(UI.el("span", "faint small", I18n.t("admin.notInGroup")));
    }
    drawerBody.appendChild(groupsWrap);

    const createBtn = document.getElementById("drawer-create-group");
    createBtn.disabled = !member.is_active;
    createBtn.title = member.is_active ? "" : I18n.t("admin.activateFirst");

    resendInviteBtn.classList.toggle("hidden", !canInvite(member));
    resendInviteBtn.title = canInvite(member) ? I18n.t("admin.resendInviteHint") : "";
    resendInviteBtn.dataset.memberId = String(member.id);

    drawer.classList.add("open");
    backdrop.classList.add("open");
  }

  function addFact(container, label, value) {
    const row = UI.el("div", "spread");
    row.appendChild(UI.el("span", "small muted", label));
    row.appendChild(UI.el("span", "small", value));
    container.appendChild(row);
  }

  async function loadAll() {
    const [members, groups] = await Promise.all([API.listMembers(), API.listGroups()]);
    state.members = members;
    state.groups = groups;
    renderMembers();
    renderGroups();
  }

  Chrome.onLanguageChange = () => {
    renderMembers();
    renderGroups();
    if (drawer.classList.contains("open") && state.drawerMemberId) {
      const memberId = state.drawerMemberId;
      openDrawer(memberId);
    }
  };

  document.getElementById("add-member").addEventListener("click", () => openMemberModal(null));
  document.getElementById("add-group").addEventListener("click", () => openGroupModal(null));
  document.getElementById("member-save").addEventListener("click", saveMember);
  document.getElementById("group-save").addEventListener("click", saveGroup);
  document.getElementById("drawer-create-group").addEventListener("click", () => {
    state.groupContextMemberId = state.drawerMemberId;
    const member = memberById(state.drawerMemberId);
    closeDrawer();
    openGroupModal(null);
    if (member) document.getElementById("g-name").value = `${member.name} & `;
  });

  resendInviteBtn.addEventListener("click", () => {
    const member = memberById(Number(resendInviteBtn.dataset.memberId));
    if (member) sendInvite(member, resendInviteBtn);
  });

  showInactive.addEventListener("change", () => {
    state.showInactive = showInactive.checked;
    renderMembers();
  });

  document.querySelectorAll("#g-split button").forEach((btn) => {
    btn.addEventListener("click", () => setGroupSplit(btn.dataset.split));
  });

  document.querySelectorAll("[data-close-modal]").forEach((btn) => btn.addEventListener("click", closeMemberModal));
  document.querySelectorAll("[data-close-group-modal]").forEach((btn) => btn.addEventListener("click", closeGroupModal));
  document.querySelectorAll("[data-close-drawer]").forEach((btn) => btn.addEventListener("click", closeDrawer));
  backdrop.addEventListener("click", closeDrawer);

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    if (memberModal.classList.contains("open")) closeMemberModal();
    else if (groupModal.classList.contains("open")) closeGroupModal();
    else if (drawer.classList.contains("open")) closeDrawer();
  });

  memberForm.addEventListener("submit", (event) => {
    event.preventDefault();
    saveMember();
  });
  document.getElementById("group-form").addEventListener("submit", (event) => {
    event.preventDefault();
    saveGroup();
  });

  Chrome.start({ onReady: loadAll });
})();
