(() => {
  const form = document.getElementById("login-form");
  const input = document.getElementById("password");
  const submit = document.getElementById("login-submit");
  const errorBox = document.getElementById("login-error");

  API.getCurrency().catch(() => {});

  function showError(message) {
    errorBox.textContent = message;
    errorBox.classList.remove("hidden");
  }

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    errorBox.classList.add("hidden");

    const password = input.value;
    if (!password) {
      showError(I18n.t("login.errRequired"));
      input.focus();
      return;
    }

    submit.disabled = true;
    submit.textContent = I18n.t("login.signingIn");
    try {
      await API.login(password);
      window.location.replace("/dashboard.html");
    } catch (err) {
      showError(err.message || I18n.t("login.errFailed"));
      input.select();
    } finally {
      submit.disabled = false;
      submit.textContent = I18n.t("login.submit");
    }
  });
})();
