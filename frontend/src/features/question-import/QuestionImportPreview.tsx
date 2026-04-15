import { useState } from "react";
import type { ParseResult, ParsedQuestion } from "./ruleParser";

type PreviewMode = "all" | "needs-review";
type QuestionFilter = "all" | ParsedQuestion["status"];

type EditingQuestion = {
  index: number;
  question: ParsedQuestion;
};

export function QuestionImportPreview({
  result,
  mode = "all",
  onQuestionSave,
}: {
  result: ParseResult;
  mode?: PreviewMode;
  onQuestionSave?: (index: number, question: ParsedQuestion) => Promise<void>;
}) {
  const { summary, questions } = result;
  const [filter, setFilter] = useState<QuestionFilter>(mode === "needs-review" ? "review" : "all");
  const [editing, setEditing] = useState<EditingQuestion | null>(null);
  const baseQuestions = questions
    .map((question, index) => ({ question, index }))
    .filter(({ question }) => mode !== "needs-review" || question.status !== "pass");
  const visibleQuestions = filter === "all"
    ? baseQuestions
    : baseQuestions.filter(({ question }) => question.status === filter);

  async function saveEditingQuestion(question: ParsedQuestion) {
    if (!editing || !onQuestionSave) return;
    await onQuestionSave(editing.index, question);
    setEditing(null);
  }

  return (
    <section className="parser-preview" aria-label="Kết quả tách câu hỏi">
      <div className="parser-scoreboard">
        <Score label="Tổng câu" value={summary.total} tone="neutral" active={filter === "all"} onClick={() => setFilter("all")} />
        <Score label="Pass" value={summary.passed} tone="pass" active={filter === "pass"} onClick={() => setFilter("pass")} />
        <Score label="Cần kiểm tra" value={summary.review} tone="review" active={filter === "review"} onClick={() => setFilter("review")} />
        <Score label="Lỗi" value={summary.failed} tone="fail" active={filter === "fail"} onClick={() => setFilter("fail")} />
      </div>

      {visibleQuestions.length === 0 ? (
        <div className="parser-empty">
          {questions.length === 0
            ? "Chưa có câu hỏi để xem. Quay lại Nguồn đề và chạy lại kiểm tra sau khi có text."
            : "Không có câu nào trong nhóm đang chọn."}
        </div>
      ) : (
        <div className="parsed-question-list">
          {visibleQuestions.map(({ question, index }) => (
            <QuestionCard
              question={question}
              key={`${index}-${question.sourceOrder}`}
              onEdit={onQuestionSave ? () => setEditing({ index, question }) : undefined}
            />
          ))}
        </div>
      )}

      {editing && (
        <QuestionEditModal
          key={`${editing.index}-${editing.question.importItemId ?? editing.question.sourceOrder}`}
          question={editing.question}
          onCancel={() => setEditing(null)}
          onSave={saveEditingQuestion}
        />
      )}
    </section>
  );
}

function Score({
  label,
  value,
  tone,
  active,
  onClick,
}: {
  label: string;
  value: number | string;
  tone: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button className={`score-card ${tone} ${active ? "active" : ""}`} type="button" onClick={onClick}>
      <span>{label}</span>
      <strong>{value}</strong>
    </button>
  );
}

function QuestionCard({ question, onEdit }: { question: ParsedQuestion; onEdit?: () => void }) {
  return (
    <article className={`parsed-question ${question.status}`}>
      <div className="parsed-question-top">
        <strong>Câu {question.sourceOrder}</strong>
        <span>
          {statusLabel(question.status)}
          {question.warnings.length > 0 && (
            <span className="warning-help" tabIndex={0} aria-label="Lý do cần kiểm tra">
              ?
              <span className="warning-popover">
                {question.warnings.map((warning) => <span key={warning}>{warning}</span>)}
              </span>
            </span>
          )}
        </span>
      </div>
      <p>{question.content || "Chưa tách được nội dung câu hỏi."}</p>
      <div className="parsed-options">
        {question.options.map((option) => (
          <span className={option.label === question.correctLabel ? "correct" : ""} key={`${question.sourceOrder}-${option.label}`}>
            {option.label}. {option.content}
          </span>
        ))}
      </div>
      {onEdit && (
        <button className="question-edit-trigger" type="button" onClick={onEdit}>
          Sửa câu này
        </button>
      )}
    </article>
  );
}

function QuestionEditModal({
  question,
  onCancel,
  onSave,
}: {
  question: ParsedQuestion;
  onCancel: () => void;
  onSave: (question: ParsedQuestion) => Promise<void>;
}) {
  const [draft, setDraft] = useState(question);
  const [saveState, setSaveState] = useState<"idle" | "saving" | "error">("idle");

  function updateDraft(patch: Partial<ParsedQuestion>) {
    setSaveState("idle");
    setDraft((current) => ({ ...current, ...patch }));
  }

  function updateOption(optionIndex: number, content: string) {
    updateDraft({
      options: draft.options.map((option, index) => index === optionIndex ? { ...option, content } : option),
    });
  }

  function addOption() {
    const label = String.fromCharCode("A".charCodeAt(0) + draft.options.length);
    updateDraft({ options: [...draft.options, { label, content: "" }] });
  }

  async function saveDraft() {
    setSaveState("saving");
    try {
      await onSave(draft);
    } catch {
      setSaveState("error");
    }
  }

  return (
    <div className="question-modal-backdrop" role="presentation">
      <section className="question-edit-modal" role="dialog" aria-modal="true" aria-label={`Sửa câu ${draft.sourceOrder}`}>
        <header>
          <div>
            <p className="eyebrow">Sửa câu hỏi</p>
            <h3>Câu {draft.sourceOrder}</h3>
          </div>
          <button className="ghost-btn" type="button" onClick={onCancel}>Đóng</button>
        </header>

        <label>
          Câu hỏi
          <textarea value={draft.content} onChange={(event) => updateDraft({ content: event.target.value })} rows={4} />
        </label>

        <div className="option-editor-list">
          {draft.options.map((option, optionIndex) => (
            <label key={option.label}>
              {option.label}
              <input value={option.content} onChange={(event) => updateOption(optionIndex, event.target.value)} />
            </label>
          ))}
        </div>

        <div className="question-editor-actions">
          <label>
            Đáp án
            <select value={draft.correctLabel || ""} onChange={(event) => updateDraft({ correctLabel: event.target.value || undefined })}>
              <option value="">Chưa chọn</option>
              {draft.options.map((option) => <option key={option.label} value={option.label}>{option.label}</option>)}
            </select>
          </label>
          <button className="ghost-btn" type="button" onClick={addOption}>Thêm lựa chọn</button>
          <button className="primary-btn" type="button" onClick={saveDraft} disabled={!draft.importItemId || saveState === "saving"}>
            {saveState === "saving" ? "Đang lưu..." : "Lưu câu"}
          </button>
        </div>
        {saveState === "error" && <p className="inline-save-note error">Không lưu được câu này.</p>}
      </section>
    </div>
  );
}

function statusLabel(status: ParsedQuestion["status"]) {
  if (status === "pass") return "Pass";
  if (status === "review") return "Cần kiểm tra";
  return "Lỗi";
}
