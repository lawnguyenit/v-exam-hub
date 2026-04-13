const questions = [
  {
    title: "Goroutine được dùng để làm gì trong Go?",
    answers: [
      "Chạy tác vụ đồng thời với chi phí nhẹ",
      "Biên dịch mã nguồn thành CSS",
      "Tạo cơ sở dữ liệu quan hệ",
      "Đóng gói template HTML",
    ],
  },
  {
    title: "Kênh channel trong Go giúp xử lý việc nào?",
    answers: [
      "Trao đổi dữ liệu giữa các goroutine",
      "Vẽ biểu đồ thống kê điểm",
      "Nén ảnh trước khi upload",
      "Tạo file CSS tự động",
    ],
  },
  {
    title: "Lệnh nào thường dùng để chạy ứng dụng Go cục bộ?",
    answers: ["go run .", "go serve ui", "npm publish", "docker style"],
  },
];

let currentQuestion = 0;
let secondsLeft = 38 * 60 + 42;

const questionMeta = document.querySelector("#questionMeta");
const questionTitle = document.querySelector("#questionTitle");
const answers = document.querySelector("#answers");
const questionPills = document.querySelectorAll(".question-pill");
const countdown = document.querySelector("#countdown");

function renderQuestion(index) {
  const question = questions[index % questions.length];
  questionMeta.textContent = `Câu ${index + 1} / 12`;
  questionTitle.textContent = question.title;
  answers.innerHTML = question.answers
    .map((answer) => `<label><input type="radio" name="answer"> ${answer}</label>`)
    .join("");

  questionPills.forEach((pill, pillIndex) => {
    pill.classList.toggle("active", pillIndex === index);
  });
}

function moveQuestion(offset) {
  currentQuestion = Math.min(11, Math.max(0, currentQuestion + offset));
  renderQuestion(currentQuestion);
}

document.querySelector("#nextQuestion").addEventListener("click", () => moveQuestion(1));
document.querySelector("#prevQuestion").addEventListener("click", () => moveQuestion(-1));

questionPills.forEach((pill, index) => {
  pill.addEventListener("click", () => {
    currentQuestion = index;
    renderQuestion(index);
  });
});

setInterval(() => {
  secondsLeft = Math.max(0, secondsLeft - 1);
  const minutes = String(Math.floor(secondsLeft / 60)).padStart(2, "0");
  const seconds = String(secondsLeft % 60).padStart(2, "0");
  countdown.textContent = `${minutes}:${seconds}`;
}, 1000);
