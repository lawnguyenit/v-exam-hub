const auth = JSON.parse(sessionStorage.getItem("examhub:auth") || "null");

if (!auth || auth.role !== "teacher") {
  window.location.replace("/");
  throw new Error("Teacher session is required");
}

const teacherExamList = document.querySelector("#teacherExamList");
const metricGrid = document.querySelector("#metricGrid");
const statsTable = document.querySelector("#statsTable");
const statMode = document.querySelector("#statMode");
const examSearch = document.querySelector("#examSearch");
const examStatusFilter = document.querySelector("#examStatusFilter");
let selectedExam = null;
let teacherExams = [];

async function loadDashboard() {
  const response = await fetch(`/api/teacher/dashboard?account=${encodeURIComponent(auth.account)}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("Cannot load teacher dashboard");
  }

  const data = await response.json();
  renderDashboard(data);
  if (data.exams.length > 0) {
    loadExamDetail(data.exams[0].id);
  }
}

function renderDashboard(data) {
  setText("#teacherName", data.profile.displayName);
  setText("#teacherDept", data.profile.department);
  setText("#teacherAvatar", (auth.account || "GV").slice(0, 2).toUpperCase());
  teacherExams = data.exams;
  renderExamList();
}

function renderExamList() {
  const query = examSearch.value.trim().toLowerCase();
  const status = examStatusFilter.value;
  let filtered = teacherExams.filter((exam) => {
    const matchQuery = !query || `${exam.title} ${exam.targetClass}`.toLowerCase().includes(query);
    const matchStatus = status === "all" || exam.status === status;
    return matchQuery && matchStatus;
  });

  if (status === "all" && !query) {
    filtered = filtered.slice(0, 3);
  }

  teacherExamList.innerHTML = filtered.map((exam) => `
    <button class="teacher-exam-card" type="button" data-exam-id="${exam.id}">
      <span class="status-badge ${statusClass(exam.status)}">${exam.status}</span>
      <h3>${exam.title}</h3>
      <div class="exam-meta">
        <span>${exam.targetClass}</span>
        <span>${exam.startTime}</span>
        <span>${exam.submitted}/${exam.total} đã nộp</span>
        <span>TB ${exam.average}</span>
      </div>
    </button>
  `).join("");

  if (filtered.length === 0) {
    teacherExamList.innerHTML = `<article class="teacher-exam-card">Không có bài thi phù hợp bộ lọc.</article>`;
    return;
  }

  teacherExamList.querySelectorAll("[data-exam-id]").forEach((button) => {
    button.addEventListener("click", () => loadExamDetail(button.dataset.examId));
  });

  document.querySelectorAll("[data-exam-id]").forEach((button) => {
    button.classList.toggle("active", selectedExam && button.dataset.examId === selectedExam.id);
  });
}

async function loadExamDetail(examID) {
  const response = await fetch(`/api/teacher/exams/${encodeURIComponent(examID)}`, { cache: "no-store" });
  if (!response.ok) {
    throw new Error("Cannot load teacher exam detail");
  }

  selectedExam = await response.json();
  document.querySelectorAll("[data-exam-id]").forEach((button) => {
    button.classList.toggle("active", button.dataset.examId === examID);
  });
  renderExamDetail();
}

function renderExamDetail() {
  if (!selectedExam) return;

  setText("#detailTitle", selectedExam.title);
  setText("#detailMeta", `${selectedExam.status} - ${selectedExam.targetClass} - ${selectedExam.startTime}`);
  metricGrid.innerHTML = selectedExam.metrics.map((metric) => `
    <article class="metric-card">
      <span>${metric.label}</span>
      <strong>${metric.value}</strong>
    </article>
  `).join("");

  renderDetailEnabledTable();
}

function renderDetailEnabledTable() {
  const { key, table } = activeStatisticsTable();
  const columns = table.columns.length + 1;
  const rows = buildDetailRows(table, key);

  statsTable.innerHTML = `
    <div class="stats-scroll">
      <div class="stats-table">
        <div class="stats-row header" style="--columns: ${columns}">
          ${table.columns.map((column) => `<span>${column}</span>`).join("")}
          <span>Chi tiết</span>
        </div>
        ${rows.map((row) => `
          <div class="stats-row" style="--columns: ${columns}">
            ${row.cells.map((cell) => `<span>${cell}</span>`).join("")}
            <button class="table-action" type="button" data-row-index="${row.rowIndex}">Xem</button>
          </div>
        `).join("")}
      </div>
    </div>
  `;

  statsTable.querySelectorAll("[data-row-index]").forEach((button) => {
    button.addEventListener("click", () => openStatDetailModal(Number(button.dataset.rowIndex)));
  });
}

function buildDetailRows(table, key) {
  if (key === "live_status") {
    return (selectedExam.students || []).map((student, index) => ({
      cells: [student.name, student.progress, student.warning],
      studentIndex: index,
      rowIndex: index,
    }));
  }

  if (key === "top_students") {
    return table.rows.map((row, index) => ({
      cells: row,
      studentIndex: findStudentIndex(row[0]),
      rowIndex: index,
    }));
  }

  return table.rows.map((row, index) => ({
    cells: row,
    studentIndex: null,
    rowIndex: index,
  }));
}

function findStudentIndex(name) {
  const index = (selectedExam.students || []).findIndex((student) => student.name === name);
  return index >= 0 ? index : 0;
}

function statusClass(status) {
  if (status === "Đang mở") return "status-open";
  if (status === "Lịch dự kiến") return "status-scheduled";
  if (status === "Thi thử") return "status-practice";
  return "status-default";
}

function openStatDetailModal(rowIndex) {
  const { key, table } = activeStatisticsTable();
  const row = buildDetailRows(table, key)[rowIndex];
  const student = row.studentIndex !== null ? selectedExam.students?.[row.studentIndex] : null;
  let content = "";

  if (student && (key === "top_students" || key === "live_status")) {
    content = renderStudentDetailContent(student);
  } else if (key === "score_distribution") {
    content = renderScoreGroupContent(table.rows[rowIndex], rowIndex, table.rows.length);
  } else if (key === "question_difficulty") {
    content = renderQuestionDifficultyContent(table.rows[rowIndex], rowIndex, table.rows.length);
  } else {
    content = renderGenericRowContent(table.columns, table.rows[rowIndex]);
  }

  openDetailModal(content);
}

function openDetailModal(content) {
  let modal = document.querySelector("#teacherDetailModal");

  if (!modal) {
    modal = document.createElement("div");
    modal.id = "teacherDetailModal";
    modal.className = "modal-backdrop";
    modal.innerHTML = `
      <section class="student-detail-modal" role="dialog" aria-modal="true" aria-labelledby="studentDetailTitle">
        <button class="modal-close" type="button" aria-label="Đóng">×</button>
        <div id="studentDetailContent"></div>
      </section>
    `;
    document.body.appendChild(modal);
    modal.addEventListener("click", (event) => {
      if (event.target === modal || event.target.classList.contains("modal-close")) {
        closeStudentAttemptModal();
      }
    });
  }

  modal.querySelector("#studentDetailContent").innerHTML = content;
  modal.classList.add("open");
}

function renderStudentDetailContent(student) {
  return `
    <p class="eyebrow">Chi tiết sinh viên</p>
    <h2 id="studentDetailTitle">${student.name}</h2>
    <div class="student-detail-meta">
      <span>Tiến trình: ${student.progress}</span>
      <span>Điểm: ${student.score}</span>
      <span>Thời gian: ${student.duration}</span>
      <span>Cảnh báo: ${student.warning}</span>
    </div>
    <div class="wrong-list">
      ${student.wrongItems.map((item) => `
        <div class="wrong-item">
          <strong>${item.question}</strong>
          <span>Đã chọn: ${item.selected}</span>
          <span>Đáp án đúng: ${item.correct}</span>
          <p>${item.note}</p>
        </div>
      `).join("")}
    </div>
  `;
}

function renderScoreGroupContent(row, rowIndex, rowCount) {
  const students = studentsForRow(rowIndex, rowCount);

  return `
    <p class="eyebrow">Chi tiết nhóm điểm</p>
    <h2 id="studentDetailTitle">Khoảng ${row[0]}</h2>
    <div class="student-detail-meta">
      <span>Số sinh viên: ${row[1]}</span>
      <span>Tỷ lệ: ${row[2]}</span>
    </div>
    ${renderStudentPreviewList(students)}
  `;
}

function renderQuestionDifficultyContent(row, rowIndex, rowCount) {
  const students = studentsForRow(rowIndex, rowCount);

  return `
    <p class="eyebrow">Chi tiết câu hỏi</p>
    <h2 id="studentDetailTitle">${row[0]} - ${row[2]}</h2>
    <div class="student-detail-meta">
      <span>Tỷ lệ sai: ${row[1]}</span>
      <span>Sinh viên liên quan: ${students.length}</span>
    </div>
    ${renderStudentPreviewList(students)}
  `;
}

function renderGenericRowContent(columns, row) {
  return `
    <p class="eyebrow">Chi tiết thống kê</p>
    <h2 id="studentDetailTitle">${row?.[0] || "Chưa có dữ liệu"}</h2>
    <div class="wrong-list">
      ${(row || []).map((cell, index) => `
        <div class="wrong-item">
          <strong>${columns[index] || "Mục"}</strong>
          <span>${cell}</span>
        </div>
      `).join("")}
    </div>
  `;
}

function renderStudentPreviewList(students) {
  if (!students.length) {
    return `<p class="empty-note">Chưa có sinh viên phù hợp hàng thống kê này.</p>`;
  }

  return `
    <div class="wrong-list">
      ${students.map((student) => `
        <div class="wrong-item">
          <strong>${student.name}</strong>
          <span>Điểm: ${student.score}</span>
          <span>Thời gian: ${student.duration}</span>
          <span>Cảnh báo: ${student.warning}</span>
          <p>${student.wrongItems?.[0]?.note || "Chưa có ghi chú sai chi tiết."}</p>
        </div>
      `).join("")}
    </div>
  `;
}

function studentsForRow(rowIndex, rowCount) {
  return (selectedExam.students || []).filter((_, index) => index % rowCount === rowIndex);
}

function activeStatisticsTable() {
  const key = selectedExam.tables[statMode.value] ? statMode.value : Object.keys(selectedExam.tables)[0];
  return { key, table: selectedExam.tables[key] };
}

function closeStudentAttemptModal() {
  document.querySelector("#teacherDetailModal")?.classList.remove("open");
}

function setText(selector, value) {
  document.querySelector(selector).textContent = value;
}

statMode.addEventListener("change", renderExamDetail);
examSearch.addEventListener("input", renderExamList);
examStatusFilter.addEventListener("change", renderExamList);
document.querySelector("#logoutLink")?.addEventListener("click", () => {
  sessionStorage.removeItem("examhub:auth");
});

loadDashboard().catch(() => {
  teacherExamList.innerHTML = `<article class="teacher-exam-card">Không thể tải dashboard giáo viên.</article>`;
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    closeStudentAttemptModal();
  }
});
