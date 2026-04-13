const auth = readJSON("examhub:auth", sessionStorage);

if (!auth || auth.role !== "student") {
  window.location.replace("/");
  throw new Error("Student session is required");
}

const availableExamList = document.querySelector("#availableExamList");
const plannedExamList = document.querySelector("#plannedExamList");
const historyTable = document.querySelector("#historyTable");
const availableCount = document.querySelector("#availableCount");
const plannedCount = document.querySelector("#plannedCount");
const latestScore = document.querySelector("#latestScore");
const savedProgress = document.querySelector("#savedProgress");
const logoutLink = document.querySelector("#logoutLink");

function readJSON(key, storage) {
  try {
    return JSON.parse(storage.getItem(key));
  } catch {
    return null;
  }
}

async function loadDashboard() {
  const response = await fetch(`/api/student/dashboard?account=${encodeURIComponent(auth.account)}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("Cannot load dashboard");
  }

  renderDashboard(await response.json());
}

function renderDashboard(data) {
  const account = auth.account || "UI-DEMO";
  const initials = account.slice(0, 2).toUpperCase() || "SV";
  const attempt = readJSON(`examhub:attempt:${account}:go-basics-demo`, localStorage);
  const answered = attempt?.answers ? Object.keys(attempt.answers).length : 0;

  setText("#studentName", data.profile.displayName);
  setText("#studentAccount", `Tài khoản: ${account}`);
  setText("#studentAvatar", initials);
  setText("#profileAvatar", initials);
  setText("#quickName", data.profile.displayName);
  setText("#quickClass", data.profile.className);
  setText("#quickCode", account.toUpperCase());
  setText("#profileName", data.profile.displayName);
  setText("#profileAccount", account);
  setText("#profileClass", data.profile.className);
  setText("#profileEmail", data.profile.email);
  setText("#profileStatus", data.profile.status);
  setText("#availableCount", data.summary.availableCount);
  setText("#plannedCount", data.summary.plannedCount);
  setText("#latestScore", data.summary.latestScore);
  setText("#savedProgress", `${answered}/12`);

  availableExamList.innerHTML = data.availableExams.map((exam, index) => `
    <article class="exam-card ${index === 0 ? "featured" : ""}">
      <div>
        <p class="eyebrow">${exam.status}</p>
        <h3>${exam.title}</h3>
        <p>${exam.meta} Thời lượng ${exam.duration}.</p>
      </div>
      <a class="${index === 0 ? "primary-btn" : "ghost-btn"}" href="/student/exam?id=${encodeURIComponent(exam.id)}">Vào làm bài</a>
    </article>
  `).join("");

  plannedExamList.innerHTML = data.plannedExams.map((exam) => `
    <article>
      <span>${exam.time}</span>
      <strong>${exam.title}</strong>
      <small>${exam.detail}</small>
    </article>
  `).join("");

  historyTable.innerHTML = `
    <div class="history-row header" role="row">
      <span>Bài thi</span>
      <span>Ngày thi</span>
      <span>Điểm</span>
      <span>Thời gian</span>
      <span></span>
    </div>
    ${data.history.map((record) => `
      <div class="history-row" role="row">
        <span>${record.title}</span>
        <span>${record.date}</span>
        <span>${record.score}</span>
        <span>${record.duration}</span>
        <a class="ghost-btn" href="/student/review?id=${encodeURIComponent(record.id)}">Xem chi tiết</a>
      </div>
    `).join("")}
  `;
}

function setText(selector, value) {
  document.querySelector(selector).textContent = value;
}

document.querySelectorAll("[data-view]").forEach((tab) => {
  tab.addEventListener("click", () => {
    document.querySelectorAll("[data-view-panel]").forEach((panel) => {
      panel.classList.toggle("active", panel.dataset.viewPanel === tab.dataset.view);
    });
    document.querySelectorAll("[data-view]").forEach((item) => {
      item.classList.toggle("active", item === tab);
    });
  });
});

logoutLink.addEventListener("click", () => {
  sessionStorage.removeItem("examhub:auth");
});

loadDashboard().catch(() => {
  availableExamList.innerHTML = `<article class="exam-card"><p>Không thể tải dashboard. Hãy thử lại sau.</p></article>`;
});
