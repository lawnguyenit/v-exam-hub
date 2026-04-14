import { useEffect, useState } from "react";
import type { Exam } from "../../api";
import { formatSeconds, formatTime } from "../../lib/format";
import { PageShell } from "../../shared/PageShell";
import type { AuthSession } from "../../storage";
import { loadAttempt } from "./attemptStorage";

export function ExamWorkspace({ auth, exam }: { auth: AuthSession; exam: Exam }) {
  const attemptKey = `examhub:attempt:${auth.account}:${exam.id}`;
  const [attempt, setAttempt] = useState(() => loadAttempt(auth, exam, attemptKey));
  const [now, setNow] = useState(Date.now());
  const currentQuestion = exam.questions[attempt.currentQuestion];
  const expired = now >= attempt.endAt;
  const secondsLeft = Math.max(0, Math.floor((attempt.endAt - now) / 1000));
  const selectedAnswer = attempt.answers[String(attempt.currentQuestion)];

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    localStorage.setItem(attemptKey, JSON.stringify(attempt));
  }, [attempt, attemptKey]);

  useEffect(() => {
    if (expired && attempt.status !== "expired") {
      setAttempt((current) => ({ ...current, status: "expired" }));
    }
  }, [attempt.status, expired]);

  function saveAnswer(answerIndex: number) {
    setAttempt((current) => ({
      ...current,
      answers: { ...current.answers, [current.currentQuestion]: answerIndex },
      lastSavedAt: Date.now(),
    }));
  }

  function moveQuestion(offset: number) {
    setAttempt((current) => ({
      ...current,
      currentQuestion: Math.min(exam.questions.length - 1, Math.max(0, current.currentQuestion + offset)),
    }));
  }

  return (
    <PageShell backTo="/student">
      <main className="exam-page">
        <section className="exam-workspace" aria-label="Bài đang làm">
          <article className="exam-panel">
            <div className="panel-head">
              <div>
                <p className="eyebrow">Bài đang làm</p>
                <h1>{exam.title}</h1>
              </div>
              <div className="timer" aria-live="polite">
                <span>Còn lại</span>
                <strong>{formatSeconds(secondsLeft)}</strong>
              </div>
            </div>
            <div className="question-box" aria-live="polite">
              <p className="question-meta">Câu {attempt.currentQuestion + 1} / {exam.questions.length}</p>
              <h2>{currentQuestion.title}</h2>
              <div className="answers">
                {currentQuestion.answers.map((answer, answerIndex) => (
                  <label key={answer}>
                    <input
                      type="radio"
                      name="answer"
                      value={answerIndex}
                      checked={selectedAnswer === answerIndex}
                      disabled={expired}
                      onChange={() => saveAnswer(answerIndex)}
                    />
                    {answer}
                  </label>
                ))}
              </div>
            </div>
            <div className="question-nav" aria-label="Danh sách câu hỏi">
              {exam.questions.map((question, index) => (
                <button
                  className={`question-pill ${index === attempt.currentQuestion ? "active" : ""} ${Object.hasOwn(attempt.answers, String(index)) ? "done" : ""}`}
                  type="button"
                  key={question.title}
                  onClick={() => setAttempt((current) => ({ ...current, currentQuestion: index }))}
                >
                  {index + 1}
                </button>
              ))}
            </div>
            <div className="exam-actions">
              <button className="ghost-btn" type="button" disabled={expired} onClick={() => moveQuestion(-1)}>Câu trước</button>
              <button className="primary-btn" type="button" disabled={expired} onClick={() => moveQuestion(1)}>Lưu và tiếp tục</button>
            </div>
          </article>
          <aside className="student-side">
            <article className="status-panel">
              <p className="eyebrow">Trạng thái</p>
              <h2>{expired ? "Đã hết thời gian" : attempt.lastSavedAt ? "Đã khôi phục tiến trình" : "Bắt đầu tiến trình mới"}</h2>
              <p>{expired ? "Tiến trình vẫn được giữ lại để đối chiếu, nhưng không nên cho sửa đáp án sau khi hết giờ." : "Tiến trình sẽ được tự động lưu khi chọn đáp án hoặc chuyển câu."}</p>
            </article>
            <article className="status-panel">
              <p className="eyebrow">Tiến trình hiện tại</p>
              <div className="metric-strip">
                <div><span>Đã trả lời</span><strong>{Object.keys(attempt.answers).length}/{exam.questions.length}</strong></div>
                <div><span>Lưu gần nhất</span><strong>{attempt.lastSavedAt ? formatTime(attempt.lastSavedAt) : "--:--"}</strong></div>
              </div>
            </article>
          </aside>
        </section>
      </main>
    </PageShell>
  );
}
