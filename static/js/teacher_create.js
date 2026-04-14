const auth = JSON.parse(sessionStorage.getItem("examhub:auth") || "null");

if (!auth || auth.role !== "teacher") {
  window.location.replace("/");
  throw new Error("Teacher session is required");
}

const form = document.querySelector("#createExamForm");
const fileInput = document.querySelector("#examFile");
const fileState = document.querySelector("#fileState");
const pipelineResult = document.querySelector("#pipelineResult");

fileInput.addEventListener("change", () => {
  const file = fileInput.files[0];
  fileState.textContent = file ? `${file.name} - ${(file.size / 1024).toFixed(1)} KB` : "Chưa có file";
});

form.addEventListener("submit", (event) => {
  event.preventDefault();

  const file = fileInput.files[0];
  if (!file) {
    pipelineResult.textContent = "Cần chọn file đề thi trước khi gửi qua AI format.";
    return;
  }

  pipelineResult.textContent = "Đã nhận file ở UI mock. Backend sau này sẽ gửi file sang pipeline AI để trích câu hỏi, đáp án, điểm và lời giải thích.";
});
