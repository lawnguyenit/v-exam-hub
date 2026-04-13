document.querySelectorAll(".login-bar").forEach((bar) => {
  bar.addEventListener("click", () => {
    sessionStorage.setItem("examhub:lastRole", bar.classList.contains("teacher") ? "teacher" : "student");
  });
});
