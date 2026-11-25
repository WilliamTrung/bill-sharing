(() => {
  const form = document.getElementById("login-form");
  const identifier = document.getElementById("identifier");
  const password = document.getElementById("password");
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

    const value = identifier.value.trim();
    const secret = password.value;
    if (!value || !secret) {
      showError(I18n.t("login.errRequired"));
      (value ? password : identifier).focus();
      return;
    }

    submit.disabled = true;
    submit.textContent = I18n.t("login.signingIn");
    try {
      await API.login(value, secret);
      window.location.replace("/dashboard.html");
    } catch (err) {
      showError(err.message || I18n.t("login.errFailed"));
      password.select();
    } finally {
      submit.disabled = false;
      submit.textContent = I18n.t("login.submit");
    }
  });
})();
