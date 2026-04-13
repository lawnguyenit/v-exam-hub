const auth = readJSON("examhub:auth", sessionStorage);

if (!auth || auth.role !== "student") {
  window.location.replace("/");
  throw new Error("Student session is required");
}

const params = new URLSearchParams(window.location.search);
const reviewID = params.get("id") || "go-intro";
const reviewTitle = document.querySelector("#reviewTitle");
const reviewScore = document.querySelector("#reviewScore");
const reviewDuration = document.querySelector("#reviewDuration");
const reviewList = document.querySelector("#reviewList");

function readJSON(key, storage) {
  try {
    return JSON.parse(storage.getItem(key));
  } catch {
    return null;
  }
}

async function loadReview() {
  const response = await fetch(`/api/student/reviews/${encodeURIComponent(reviewID)}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("Cannot load review");
  }

  renderReview(await response.json());
}

function renderReview(review) {
  reviewTitle.textContent = review.title;
  reviewScore.textContent = review.score;
  reviewDuration.textContent = `Thời gian làm bài: ${review.duration}`;
  reviewList.innerHTML = review.questions.map((question, index) => `
    <article class="review-question">
      <p class="eyebrow">Câu ${index + 1}</p>
      <h2>${question.title}</h2>
      <div class="review-answers">
        ${question.answers.map((answer, answerIndex) => {
          const correct = answerIndex === question.correctAnswer;
          const selected = answerIndex === question.selectedAnswer;
          const wrongSelected = selected && !correct;
          const className = correct ? "correct" : wrongSelected ? "wrong" : "";
          const suffix = correct ? "Đáp án đúng" : selected ? "Bạn đã chọn" : "";
          return `<div class="review-answer ${className}"><span>${String.fromCharCode(65 + answerIndex)}. ${answer}</span><strong>${suffix}</strong></div>`;
        }).join("")}
      </div>
    </article>
  `).join("");
}

loadReview().catch(() => {
  reviewTitle.textContent = "Không thể tải bài xem lại";
});
