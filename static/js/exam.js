const auth = readJSON("examhub:auth", sessionStorage);

if (!auth || auth.role !== "student") {
  window.location.replace("/");
  throw new Error("Student session is required");
}

const params = new URLSearchParams(window.location.search);
const examID = params.get("id") || "go-basics-demo";
const attemptKey = `examhub:attempt:${auth.account}:${examID}`;
const examTitle = document.querySelector("#examTitle");
const questionMeta = document.querySelector("#questionMeta");
const questionTitle = document.querySelector("#questionTitle");
const answers = document.querySelector("#answers");
const questionNav = document.querySelector("#questionNav");
const countdown = document.querySelector("#countdown");
const attemptStateTitle = document.querySelector("#attemptStateTitle");
const attemptStateText = document.querySelector("#attemptStateText");
const answeredCount = document.querySelector("#answeredCount");
const lastSavedAt = document.querySelector("#lastSavedAt");

let exam = null;
let attempt = null;

function readJSON(key, storage) {
  try {
    return JSON.parse(storage.getItem(key));
  } catch {
    return null;
  }
}

async function initExam() {
  const response = await fetch(`/api/student/exams/${encodeURIComponent(examID)}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("Cannot load exam");
  }

  exam = await response.json();
  attempt = loadAttempt();
  examTitle.textContent = exam.title;
  questionNav.innerHTML = exam.questions.map((_, index) => `<button class="question-pill" type="button">${index + 1}</button>`).join("");

  questionNav.querySelectorAll(".question-pill").forEach((pill, index) => {
    pill.addEventListener("click", () => {
      attempt.currentQuestion = index;
      renderQuestion(index);
    });
  });

  attemptStateTitle.textContent = attempt.lastSavedAt ? "Đã khôi phục tiến trình" : "Bắt đầu tiến trình mới";
  attemptStateText.textContent = attempt.lastSavedAt
    ? "Bài làm được mở lại từ dữ liệu đã lưu trên máy này. Thời gian vẫn tính theo mốc kết thúc ban đầu."
    : "Tiến trình sẽ được lưu cục bộ khi chọn đáp án hoặc chuyển câu.";
  renderQuestion(attempt.currentQuestion);
  updateTimer();
  setInterval(updateTimer, 1000);
}

function loadAttempt() {
  const saved = readJSON(attemptKey, localStorage);
  const now = Date.now();

  if (saved && saved.examId === exam.id) {
    return {
      ...saved,
      answers: saved.answers || {},
      currentQuestion: Number.isInteger(saved.currentQuestion) ? saved.currentQuestion : 0,
    };
  }

  const initial = {
    examId: exam.id,
    account: auth.account,
    startedAt: now,
    endAt: now + exam.durationSeconds * 1000,
    currentQuestion: 0,
    answers: {},
    lastSavedAt: null,
    status: "in_progress",
  };
  localStorage.setItem(attemptKey, JSON.stringify(initial));
  return initial;
}

function saveAttempt() {
  attempt.lastSavedAt = Date.now();
  localStorage.setItem(attemptKey, JSON.stringify(attempt));
  updateProgress();
}

function renderQuestion(index) {
  const question = exam.questions[index];
  const savedAnswer = attempt.answers[index];

  attempt.currentQuestion = index;
  questionMeta.textContent = `Câu ${index + 1} / ${exam.questions.length}`;
  questionTitle.textContent = question.title;
  answers.innerHTML = question.answers.map((answer, answerIndex) => {
    const checked = savedAnswer === answerIndex ? "checked" : "";
    return `<label><input type="radio" name="answer" value="${answerIndex}" ${checked}> ${answer}</label>`;
  }).join("");

  answers.querySelectorAll("input").forEach((input) => {
    input.disabled = isExpired();
    input.addEventListener("change", () => {
      attempt.answers[index] = Number(input.value);
      saveAttempt();
      renderQuestion(index);
    });
  });

  questionNav.querySelectorAll(".question-pill").forEach((pill, pillIndex) => {
    pill.classList.toggle("active", pillIndex === index);
    pill.classList.toggle("done", Object.prototype.hasOwnProperty.call(attempt.answers, pillIndex));
  });

  localStorage.setItem(attemptKey, JSON.stringify(attempt));
  updateProgress();
}

function moveQuestion(offset) {
  attempt.currentQuestion = Math.min(exam.questions.length - 1, Math.max(0, attempt.currentQuestion + offset));
  renderQuestion(attempt.currentQuestion);
}

function isExpired() {
  return Date.now() >= attempt.endAt;
}

function updateTimer() {
  const secondsLeft = Math.max(0, Math.floor((attempt.endAt - Date.now()) / 1000));
  const minutes = String(Math.floor(secondsLeft / 60)).padStart(2, "0");
  const seconds = String(secondsLeft % 60).padStart(2, "0");
  countdown.textContent = `${minutes}:${seconds}`;

  if (secondsLeft === 0) {
    attempt.status = "expired";
    attemptStateTitle.textContent = "Đã hết thời gian";
    attemptStateText.textContent = "Tiến trình vẫn được giữ lại để đối chiếu, nhưng không nên cho sửa đáp án sau khi hết giờ.";
    document.querySelector("#nextQuestion").disabled = true;
    document.querySelector("#prevQuestion").disabled = true;
    answers.querySelectorAll("input").forEach((input) => {
      input.disabled = true;
    });
    localStorage.setItem(attemptKey, JSON.stringify(attempt));
  }
}

function updateProgress() {
  const answered = Object.keys(attempt.answers).length;
  answeredCount.textContent = `${answered}/${exam.questions.length}`;
  lastSavedAt.textContent = attempt.lastSavedAt
    ? new Date(attempt.lastSavedAt).toLocaleTimeString("vi-VN", { hour: "2-digit", minute: "2-digit" })
    : "--:--";
}

document.querySelector("#nextQuestion").addEventListener("click", () => moveQuestion(1));
document.querySelector("#prevQuestion").addEventListener("click", () => moveQuestion(-1));

initExam().catch(() => {
  examTitle.textContent = "Không thể tải bài kiểm tra";
});
