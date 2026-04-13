const auth = JSON.parse(sessionStorage.getItem("examhub:auth") || "null");

if (!auth || auth.role !== "teacher") {
  window.location.replace("/");
  throw new Error("Teacher session is required");
}

document.querySelector("#logoutLink").addEventListener("click", () => {
  sessionStorage.removeItem("examhub:auth");
});
