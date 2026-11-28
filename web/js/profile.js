(() => {
  const profileForm = document.getElementById("profile-form");
  const displayName = document.getElementById("p-display-name");
  const phone = document.getElementById("p-phone");
  const notes = document.getElementById("p-notes");
  const username = document.getElementById("p-username");
  const email = document.getElementById("p-email");
  const language = document.getElementById("p-language");
  const isPublic = document.getElementById("p-public");
  const profileSubmit = document.getElementById("profile-submit");
  const profileError = document.getElementById("profile-error");

  const passwordForm = document.getElementById("password-form");
  const currentPassword = document.getElementById("p-current");
  const newPassword = document.getElementById("p-new");
  const confirmPassword = document.getElementById("p-confirm");
  const passwordSubmit = document.getElementById("password-submit");
  const passwordError = document.getElementById("password-error");
  const passwordUnavailable = document.getElementById("password-unavailable");

  function showError(box, message) {
    box.textContent = message;
    box.classList.remove("hidden");
  }

  function clearError(box) {
    box.classList.add("hidden");
    box.textContent = "";
  }

  function fillForm(profile) {
    displayName.value = profile.display_name || "";
    phone.value = profile.phone || "";
    notes.value = profile.notes || "";
    username.value = profile.username || "";
    email.value = profile.email || "";
    language.value = profile.language === "vi" ? "vi" : "en";
    isPublic.checked = profile.is_public === true;

    const hasPassword = profile.has_password !== false;
    passwordForm.querySelectorAll("input").forEach((input) => {
      input.disabled = !hasPassword;
    });
    passwordSubmit.disabled = !hasPassword;
    passwordUnavailable.classList.toggle("hidden", hasPassword);
  }

  profileForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    clearError(profileError);

    const name = displayName.value.trim();
    const user = username.value.trim();
    const mail = email.value.trim();
    if (!name || !user || !mail) {
      showError(profileError, I18n.t("profile.errRequired"));
      (!name ? displayName : !user ? username : email).focus();
      return;
    }

    profileSubmit.disabled = true;
    profileSubmit.textContent = I18n.t("profile.saving");
    try {
      const profile = await API.updateProfile(
        name,
        user,
        mail,
        language.value,
        isPublic.checked,
        phone.value.trim(),
        notes.value.trim()
      );
      fillForm(profile);
      I18n.useDefault(profile.language);
      UI.toast(I18n.t("profile.saved"), "success");
    } catch (err) {
      showError(profileError, UI.errorMessage(err));
    } finally {
      profileSubmit.disabled = false;
      profileSubmit.textContent = I18n.t("profile.save");
    }
  });

  passwordForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    clearError(passwordError);

    const current = currentPassword.value;
    const next = newPassword.value;
    const confirm = confirmPassword.value;
    if (!current || !next) {
      showError(passwordError, I18n.t("profile.errPasswordRequired"));
      (!current ? currentPassword : newPassword).focus();
      return;
    }
    if (next !== confirm) {
      showError(passwordError, I18n.t("profile.errMismatch"));
      confirmPassword.select();
      return;
    }
    if (next.length < 8) {
      showError(passwordError, I18n.t("profile.errTooShort"));
      newPassword.focus();
      return;
    }

    passwordSubmit.disabled = true;
    passwordSubmit.textContent = I18n.t("profile.changing");
    try {
      await API.changePassword(current, next);
      passwordForm.reset();
      UI.toast(I18n.t("profile.passwordChanged"), "success");
    } catch (err) {
      showError(passwordError, UI.errorMessage(err));
    } finally {
      passwordSubmit.disabled = false;
      passwordSubmit.textContent = I18n.t("profile.changePassword");
    }
  });

  Chrome.start({
    async onReady() {
      fillForm(await API.profile());
    },
  });
})();
