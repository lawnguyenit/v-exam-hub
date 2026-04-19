import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { AttemptState, Exam } from "../../api";
import { startStudentAttempt, submitStudentAttempt, syncStudentAttemptDraft } from "../../api";
import { formatSeconds } from "../../lib/format";
import { PageShell } from "../../shared/PageShell";
import { RichQuestionText } from "../../shared/RichQuestionText";
import type { AuthSession } from "../../storage";

export function ExamWorkspace({ auth, exam }: { auth: AuthSession; exam: Exam }) {
  const navigate = useNavigate();
  const [attempt, setAttempt] = useState<AttemptState | null>(null);
  const [now, setNow] = useState(Date.now());
  const [message, setMessage] = useState("Đang khôi phục bài làm từ database...");
  const [, setDirty] = useState(false);
  const [, setIsSyncing] = useState(false);
  const [, setLastSavedAt] = useState(0);
  const [accessCode, setAccessCode] = useState("");
  const [needsAccessCode, setNeedsAccessCode] = useState(false);
  const [confirmSubmit, setConfirmSubmit] = useState(false);
  const [redirectCountdown, setRedirectCountdown] = useState<number | null>(null);
  const [flaggedQuestions, setFlaggedQuestions] = useState<Set<number>>(() => new Set());
  const [questionFilter, setQuestionFilter] = useState<"all" | "todo" | "done" | "flagged">("all");
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
    if (confirmSubmit) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [confirmSubmit]);

  useEffect(() => {
    if (redirectCountdown === null) return;
    if (redirectCountdown <= 0) {
      navigate("/student", { replace: true });
      return;
    }
    const timer = window.setTimeout(() => {
      setRedirectCountdown((current) => current === null ? null : current - 1);
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [navigate, redirectCountdown]);

  useEffect(() => {
    attemptRef.current = attempt;
    if (attempt?.lastSavedAt) {
      setLastSavedAt(attempt.lastSavedAt);
    }
  }, [attempt]);

  const beginAttempt = useCallback((code = "") => {
    setMessage("Đang kiểm tra quyền vào bài...");
    return startStudentAttempt({ account: auth.account, examId: exam.id, accessCode: code })
      .then((state) => {
        setAttempt(state);
        setNeedsAccessCode(false);
        setRedirectCountdown(null);
        dirtyRef.current = false;
        setDirty(false);
        setLastSavedAt(state.lastSavedAt || 0);
        setMessage(state.status === "in_progress" ? "Đã khôi phục tiến trình từ database." : "Bài làm đã kết thúc.");
      })
      .catch((error) => {
        const text = error instanceof Error ? error.message : "Không bắt đầu được bài làm.";
        const lowerText = text.toLowerCase();
        if (lowerText.includes("ma truy cap") || lowerText.includes("mã truy cập") || lowerText.includes("access code")) {
          setNeedsAccessCode(true);
          setRedirectCountdown(null);
        } else {
          setRedirectCountdown(5);
        }
        setMessage(text);
      });
  }, [auth.account, exam.id, exam.requiresAccessCode]);

  useEffect(() => {
    let cancelled = false;
    beginAttempt("").then(() => {
      if (cancelled) return;
    });
    return () => { cancelled = true; };
  }, [beginAttempt]);

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
    setConfirmSubmit(false);
    setMessage("Đang nộp bài và chấm điểm...");
    try {
      const synced = await syncDraft("submit");
      if (!synced) return;
      const state = await submitStudentAttempt({ attemptId: synced.attemptId });
      setAttempt(state);
      dirtyRef.current = false;
      setDirty(false);
      setMessage(`Đã nộp bài. Điểm: ${state.score || "0"}.`);
      navigate("/student?view=history", { replace: true });
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Không nộp được bài.");
    }
  }

  if (!attempt || !currentQuestion) {
    return (
      <PageShell backTo="/student">
        <main className="exam-page exam-entry-page">
          <article className="exam-panel exam-entry-panel">
            {needsAccessCode ? (
              <form
                className="access-code-panel"
                onSubmit={(event) => {
                  event.preventDefault();
                  void beginAttempt(accessCode);
                }}
              >
                <p className="eyebrow">Mã truy cập</p>
                <h1>{exam.title}</h1>
                <p>Bài chính thức hoặc điểm danh cần mã do giáo viên tạo trước giờ làm.</p>
                <input
                  value={accessCode}
                  onChange={(event) => setAccessCode(event.target.value.toUpperCase())}
                  placeholder="Nhập mã truy cập"
                  autoFocus
                />
                <button className="primary-btn" type="submit">Vào bài</button>
                <p>{message}</p>
              </form>
            ) : (
              <div className="exam-message-panel">
                <p>{message}</p>
                {redirectCountdown !== null && (
                  <p className="exam-return-note">Tự quay lại sau {redirectCountdown} giây.</p>
                )}
              </div>
            )}
          </article>
        </main>
      </PageShell>
    );
  }
  const activeAttempt = attempt;

  function answeredCount() {
    return Object.keys(activeAttempt.answers).length;
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

  function isFlagged(index: number) {
    return flaggedQuestions.has(index);
  }

  function toggleFlag(index: number) {
    setFlaggedQuestions((current) => {
      const next = new Set(current);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  }

  const visibleQuestionIndexes = exam.questions
    .map((_, index) => index)
    .filter((index) => {
      if (questionFilter === "todo") return !isAnswered(index);
      if (questionFilter === "done") return isAnswered(index);
      if (questionFilter === "flagged") return isFlagged(index);
      return true;
    });

  return (
    <PageShell backTo="/student">
      <main className="exam-page">
        <section className="exam-workspace exam-workspace-focused" aria-label="Bài đang làm">
          <aside className="question-index-panel" aria-label="Danh sách câu hỏi">
            <div>
              <p className="eyebrow">Câu hỏi</p>
              <label className="question-filter-select">
                Hiển thị
                <select value={questionFilter} onChange={(event) => setQuestionFilter(event.target.value as typeof questionFilter)}>
                  <option value="all">Tất cả câu</option>
                  <option value="todo">Chưa làm</option>
                  <option value="done">Đã làm</option>
                  <option value="flagged">Đặt cờ</option>
                </select>
              </label>
            </div>
            <div className="question-nav">
              {visibleQuestionIndexes.map((index) => (
                <button
                  className={`question-pill ${index === currentIndex ? "active" : ""} ${isAnswered(index) ? "done" : ""} ${isFlagged(index) ? "flagged" : ""}`}
                  type="button"
                  key={`${exam.questions[index].title}-${index}`}
                  onClick={() => setQuestion(index)}
                >
                  {index + 1}
                </button>
              ))}
            </div>
            {visibleQuestionIndexes.length === 0 && <p className="question-empty">Không có câu phù hợp bộ lọc.</p>}
          </aside>
          <article className="exam-panel">
            <div className="panel-head">
              <div>
                <p className="eyebrow">Bài đang làm</p>
                <h1>{exam.title}</h1>
              </div>
            </div>
            <div className="question-box" aria-live="polite">
              <div className="question-box-head">
                <p className="question-meta">Câu {currentIndex + 1} / {exam.questions.length}</p>
                <button
                  className={`question-flag-btn ${isFlagged(currentIndex) ? "active" : ""}`}
                  type="button"
                  onClick={() => toggleFlag(currentIndex)}
                >
                  {isFlagged(currentIndex) ? "Bỏ cờ" : "Đặt cờ"}
                </button>
              </div>
              <h2><RichQuestionText text={currentQuestion.title} assetBatchId={currentQuestion.assetBatchId} /></h2>
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
                    <RichQuestionText text={answer} assetBatchId={currentQuestion.assetBatchId} />
                  </label>
                ))}
              </div>
            </div>
            <div className="exam-actions">
              <button className="ghost-btn" type="button" disabled={expired} onClick={() => moveQuestion(-1)}>Câu trước</button>
              <button className="primary-btn" type="button" disabled={expired} onClick={() => moveQuestion(1)}>Câu tiếp theo</button>
            </div>
          </article>
          <aside className="exam-control-panel" aria-label="Điều khiển bài làm">
            <div className="timer" aria-live="polite">
              <span>Còn lại</span>
              <strong>{formatSeconds(secondsLeft)}</strong>
            </div>
            <div className="exam-progress-mini">
              <span>Đã trả lời</span>
              <strong>{answeredCount()}/{exam.questions.length}</strong>
            </div>
            <button className="primary-btn submit-btn" type="button" disabled={expired} onClick={() => setConfirmSubmit(true)}>Nộp bài</button>
          </aside>
        </section>
        {confirmSubmit && (
          <div className="modal-backdrop open" role="dialog" aria-modal="true" aria-label="Xác nhận nộp bài">
            <section className="submit-confirm-modal">
              <p className="eyebrow">Xác nhận nộp bài</p>
              <h2>Bạn có chắc chắn muốn nộp bài?</h2>
              <p>Sau khi xác nhận, bài sẽ được chấm và chuyển sang mục lịch sử để xem điểm và chi tiết.</p>
              <div className="submit-confirm-meta">
                <span>Đã trả lời: {answeredCount()}/{exam.questions.length}</span>
                <span>Thời gian còn lại: {formatSeconds(secondsLeft)}</span>
              </div>
              <div className="modal-actions">
                <button className="ghost-btn" type="button" onClick={() => {
                  setNow(Date.now());
                  setConfirmSubmit(false);
                }}>Huỷ</button>
                <button className="primary-btn" type="button" onClick={submitAttempt}>Xác nhận nộp</button>
              </div>
            </section>
          </div>
        )}
      </main>
    </PageShell>
  );
}
