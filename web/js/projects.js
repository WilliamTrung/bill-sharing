(() => {
  const grid = document.getElementById("project-grid");
  const empty = document.getElementById("projects-empty");
  const newBtn = document.getElementById("new-project");

  let projects = [];

  function render() {
    grid.replaceChildren();
    if (projects.length === 0) {
      empty.classList.remove("hidden");
      return;
    }
    empty.classList.add("hidden");

    for (const project of projects) {
      const card = document.createElement("a");
      card.className = "card project-card";
      card.href = `/p/${project.id}/dashboard.html`;

      const head = UI.el("div", "spread");
      head.appendChild(UI.el("div", "strong", project.name));
      if (project.role === "owner") {
        head.appendChild(UI.el("span", "pill", I18n.t("projects.owner")));
      }
      card.appendChild(head);

      const count = project.member_count || 0;
      card.appendChild(
        UI.el(
          "div",
          "muted small",
          `${count} ${I18n.t(count === 1 ? "unit.member" : "unit.members")}`
        )
      );
      grid.appendChild(card);
    }
  }

  async function load() {
    projects = await API.listProjects();
    render();
  }

  async function createProject() {
    const name = await Popup.prompt({
      title: I18n.t("projects.new"),
      label: I18n.t("projects.newPrompt"),
      confirmLabel: I18n.t("action.create"),
      icon: "plus",
    });
    if (name === null) return;
    const trimmed = name.trim();
    if (!trimmed) return;
    try {
      const project = await API.createProject(trimmed);
      window.location.href = `/p/${project.id}/dashboard.html`;
    } catch (err) {
      UI.toast(UI.errorMessage(err), "error");
    }
  }

  newBtn.addEventListener("click", createProject);
  Chrome.onLanguageChange = render;

  Chrome.start({ onReady: load });
})();
