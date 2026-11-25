(() => {
  const token = new URLSearchParams(window.location.search).get("token") || "";
  const form = document.getElementById("activate-form");
  const sub = document.getElementById("activate-sub");
  const password = document.getElementById("activate-password");
  const confirm = document.getElementById("activate-confirm");
  const submit = document.getElementById("activate-submit");
  const errorBox = document.getElementById("activate-error");
  const done = document.getElementById("activate-done");

  function showError(message) {
    errorBox.textContent = message;
    errorBox.classList.remove("hidden");
  }

  function disable(message) {
    form.hidden = true;
    done.hidden = true;
    if (message) sub.textContent = message;
  }

  API.getCurrency().catch(() => {});

  if (!token) {
    disable(I18n.t("activate.missingToken"));
    return;
  }

  API.getInvite(token)
    .then((invite) => {
      sub.textContent = I18n.t("activate.greeting", { name: invite.display_name || invite.username });
    })
    .catch((err) => {
      disable(err.status === 410 ? I18n.t("activate.used") : I18n.t("activate.invalid"));
    });

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    errorBox.classList.add("hidden");

    const value = password.value;
    if (value.length < 8) {
      showError(I18n.t("activate.errTooShort"));
      password.focus();
      return;
    }
    if (value !== confirm.value) {
      showError(I18n.t("activate.errMismatch"));
      confirm.select();
      return;
    }

    submit.disabled = true;
    submit.textContent = I18n.t("activate.activating");
    try {
      await API.acceptInvite(token, value);
      form.hidden = true;
      done.hidden = false;
      UI.toast(I18n.t("activate.success"), "success");
    } catch (err) {
      showError(UI.errorMessage(err));
      submit.disabled = false;
      submit.textContent = I18n.t("activate.submit");
    }
  });
})();
