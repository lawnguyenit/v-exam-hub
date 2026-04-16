import { useCallback, useEffect, useRef, useState } from "react";
import type { AttemptState, Exam } from "../../api";
import { startStudentAttempt, submitStudentAttempt, syncStudentAttemptDraft } from "../../api";
import { formatSeconds, formatTime } from "../../lib/format";
import { PageShell } from "../../shared/PageShell";
import type { AuthSession } from "../../storage";

export function ExamWorkspace({ auth, exam }: { auth: AuthSession; exam: Exam }) {
  const [attempt, setAttempt] = useState<AttemptState | null>(null);
  const [now, setNow] = useState(Date.now());
  const [message, setMessage] = useState("Đang khôi phục bài làm từ database...");
  const [dirty, setDirty] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [lastSavedAt, setLastSavedAt] = useState(0);
  const attemptRef = useRef<AttemptState | null>(null);
  const dirtyRef = useRef(false);
  const dirtyVersionRef = useRef(0);
  const syncPromiseRef = useRef<Promise<AttemptState | null> | null>(null);
  const currentIndex = Math.min(attempt?.currentQuestion || 0, Math.max(0, exam.questions.length - 1));
  const currentQuestion = exam.questions[currentIndex];
  const expired = attempt ? now >= attempt.endAt || attempt.status !== "in_progress" : false;
  const secondsLeft = attempt ? Math.max(0, Math.floor((attempt.endAt - now) / 1000)) : 0;
  const selectedAnswer = attempt?.answers[String(currentIndex)];

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    attemptRef.current = attempt;
    if (attempt?.lastSavedAt) {
      setLastSavedAt(attempt.lastSavedAt);
    }
  }, [attempt]);

  useEffect(() => {
    let cancelled = false;
    startStudentAttempt({ account: auth.account, examId: exam.id })
      .then((state) => {
        if (cancelled) return;
        setAttempt(state);
        dirtyRef.current = false;
        setDirty(false);
        setLastSavedAt(state.lastSavedAt || 0);
        setMessage(state.status === "in_progress" ? "Đã khôi phục tiến trình từ database." : "Bài làm đã kết thúc.");
      })
      .catch((error) => {
        if (cancelled) return;
        setMessage(error instanceof Error ? error.message : "Không bắt đầu được bài làm.");
      });
    return () => { cancelled = true; };
  }, [auth.account, exam.id]);

  useEffect(() => {
    if (attempt && expired && attempt.status === "in_progress") {
      setAttempt((current) => current ? { ...current, status: "expired" } : current);
    }
  }, [attempt, expired]);

  function markDirty() {
    dirtyRef.current = true;
    dirtyVersionRef.current += 1;
    setDirty(true);
  }

  const syncDraft = useCallback(async (reason: "auto" | "submit" = "auto") => {
    if (syncPromiseRef.current) {
      await syncPromiseRef.current;
      if (reason !== "submit" || !dirtyRef.current) return attemptRef.current;
    }

    const draft = attemptRef.current;
    if (!draft || draft.status !== "in_progress" || !dirtyRef.current) return draft;

    const syncVersion = dirtyVersionRef.current;
    const syncPromise = (async () => {
      setIsSyncing(true);
      setMessage(reason === "submit" ? "Đang lưu nháp trước khi nộp bài..." : "Đang tự động lưu nháp vào database...");
      try {
        const state = await syncStudentAttemptDraft({
          attemptId: draft.attemptId,
          currentQuestion: draft.currentQuestion,
          answers: draft.answers,
        });
        setLastSavedAt(state.lastSavedAt || Date.now());
        if (dirtyVersionRef.current === syncVersion) {
          dirtyRef.current = false;
          setDirty(false);
          setAttempt(state);
          setMessage("Đã đồng bộ nháp vào database.");
        } else {
          setAttempt((current) => current ? {
            ...current,
            endAt: state.endAt,
            lastSavedAt: state.lastSavedAt,
            score: state.score,
            status: state.status,
          } : state);
          setMessage("Đã lưu một phần nháp. Các thay đổi mới sẽ lưu ở lần autosave kế tiếp.");
        }
        return state;
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "Không đồng bộ được bài làm.");
        return null;
      } finally {
        setIsSyncing(false);
      }
    })();

    syncPromiseRef.current = syncPromise;
    try {
      return await syncPromise;
    } finally {
      if (syncPromiseRef.current === syncPromise) {
        syncPromiseRef.current = null;
      }
    }
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void syncDraft("auto");
    }, 60_000);
    return () => window.clearInterval(timer);
  }, [syncDraft]);

  useEffect(() => {
    const saveOnLeave = () => {
      const draft = attemptRef.current;
      if (!draft || draft.status !== "in_progress" || !dirtyRef.current) return;
      const body = JSON.stringify({
        attemptId: draft.attemptId,
        currentQuestion: draft.currentQuestion,
        answers: draft.answers,
      });
      const blob = new Blob([body], { type: "application/json" });
      if (navigator.sendBeacon) {
        navigator.sendBeacon("/api/student/attempts/sync", blob);
      } else {
        void fetch("/api/student/attempts/sync", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body,
          keepalive: true,
        });
      }
    };
    window.addEventListener("pagehide", saveOnLeave);
    return () => window.removeEventListener("pagehide", saveOnLeave);
  }, []);

  function saveAnswer(answerIndex: number) {
    if (!attempt) return;
    setAttempt((current) => current ? {
      ...current,
      answers: { ...current.answers, [currentIndex]: answerIndex },
      currentQuestion: currentIndex,
    } : current);
    markDirty();
    setMessage("Đã ghi vào bộ đệm. Hệ thống sẽ tự lưu định kỳ hoặc khi nộp bài.");
  }

  function moveQuestion(offset: number) {
    const nextIndex = Math.min(exam.questions.length - 1, Math.max(0, activeAttempt.currentQuestion + offset));
    setQuestion(nextIndex);
  }

  async function submitAttempt() {
    if (!attempt) return;
    setMessage("Đang nộp bài và chấm điểm...");
    try {
      const synced = await syncDraft("submit");
      if (!synced) return;
      const state = await submitStudentAttempt({ attemptId: synced.attemptId });
      setAttempt(state);
      dirtyRef.current = false;
      setDirty(false);
      setMessage(`Đã nộp bài. Điểm: ${state.score || "0"}.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Không nộp được bài.");
    }
  }

  if (!attempt || !currentQuestion) {
    return (
      <PageShell backTo="/student">
        <main className="exam-page">
          <article className="exam-panel">{message}</article>
        </main>
      </PageShell>
    );
  }
  const activeAttempt = attempt;

  function answeredCount() {
    return Object.keys(activeAttempt.answers).length;
  }

  function lastSavedText() {
    return lastSavedAt ? formatTime(lastSavedAt) : "--:--";
  }

  function setQuestion(index: number) {
    setAttempt((current) => current ? {
      ...current,
      currentQuestion: index,
    } : current);
    markDirty();
  }

  function isAnswered(index: number) {
    return Object.hasOwn(activeAttempt.answers, String(index));
  }

  function statusTitle() {
    if (activeAttempt.status === "submitted") return "Đã nộp bài";
    if (expired) return "Đã hết thời gian";
    return "Đã khôi phục tiến trình";
  }

  function statusBody() {
    if (activeAttempt.status === "submitted") return `Bài đã được chấm và lưu vào database. Điểm: ${activeAttempt.score || "0"}.`;
    if (expired) return "Tiến trình vẫn được giữ lại để đối chiếu, nhưng không nên cho sửa đáp án sau khi hết giờ.";
    return "Đáp án được giữ trong bộ đệm và đồng bộ vào database mỗi 1 phút hoặc khi nộp bài.";
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
              <p className="question-meta">Câu {currentIndex + 1} / {exam.questions.length}</p>
              <h2>{currentQuestion.title}</h2>
              <div className="answers">
                {currentQuestion.answers.map((answer, answerIndex) => (
                  <label key={`${answer}-${answerIndex}`}>
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
                  className={`question-pill ${index === currentIndex ? "active" : ""} ${isAnswered(index) ? "done" : ""}`}
                  type="button"
                  key={`${question.title}-${index}`}
                  onClick={() => setQuestion(index)}
                >
                  {index + 1}
                </button>
              ))}
            </div>
            <div className="exam-actions">
              <button className="ghost-btn" type="button" disabled={expired} onClick={() => moveQuestion(-1)}>Câu trước</button>
              <button className="primary-btn" type="button" disabled={expired} onClick={() => moveQuestion(1)}>Câu tiếp theo</button>
              <button className="ghost-btn" type="button" disabled={expired} onClick={submitAttempt}>Nộp bài</button>
            </div>
          </article>
          <aside className="student-side">
            <article className="status-panel">
              <p className="eyebrow">Trạng thái</p>
              <h2>{statusTitle()}</h2>
              <p>{statusBody()}</p>
              <p>{message}</p>
            </article>
            <article className="status-panel">
              <p className="eyebrow">Tiến trình hiện tại</p>
              <div className="metric-strip">
                <div><span>Đã trả lời</span><strong>{answeredCount()}/{exam.questions.length}</strong></div>
                <div><span>Lưu gần nhất</span><strong>{dirty ? "Chưa đồng bộ" : lastSavedText()}</strong></div>
                <div><span>Autosave</span><strong>{isSyncing ? "Đang lưu" : "60 giây/lần"}</strong></div>
              </div>
            </article>
          </aside>
        </section>
      </main>
    </PageShell>
  );
}
