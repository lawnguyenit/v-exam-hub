const credentialForm = document.querySelector("#credentialForm");
const accountInput = document.querySelector("#accountInput");
const passwordInput = document.querySelector("#passwordInput");
const formMessage = document.querySelector("#formMessage");
const selectedRole = document.body.dataset.role;

if (!credentialForm || !selectedRole) {
  throw new Error("Login form requires a selected role");
}

sessionStorage.setItem("examhub:lastRole", selectedRole);

credentialForm.addEventListener("submit", (event) => {
  event.preventDefault();

  const account = accountInput.value.trim();
  const password = passwordInput.value.trim();

  if (!account || !password) {
    formMessage.textContent = "Cần nhập đủ tài khoản và mật khẩu được cấp.";
    return;
  }

  sessionStorage.setItem("examhub:auth", JSON.stringify({
    account,
    role: selectedRole,
    signedInAt: Date.now(),
  }));

  window.location.href = selectedRole === "teacher" ? "/teacher" : "/student";
});
