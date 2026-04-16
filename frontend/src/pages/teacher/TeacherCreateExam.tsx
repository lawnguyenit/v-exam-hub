import type { FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { approveTeacherImportPassItems, getTeacherClasses, parseTeacherImport, saveTeacherImportItem } from "../../api";
import { QuestionImportPreview } from "../../features/question-import/QuestionImportPreview";
import { parseExamText, recheckParsedQuestions } from "../../features/question-import/ruleParser";
import type { ParsedQuestion, ParseResult } from "../../features/question-import/ruleParser";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

const emptyFileState = "Chưa có file";

const acceptedFormats = ["TXT", "CSV", "DOCX", "DOC cũ", "PDF text", "PDF scan"];

const pipelineSteps = [
  ["Nhận file", "Lưu file gốc và metadata để đối chiếu."],
  ["Tách text", "TXT/CSV đọc trực tiếp. DOC/DOCX/PDF do backend tách text hoặc OCR."],
  ["Tách câu tự động", "Nhận dạng câu hỏi, lựa chọn và đáp án bằng luật mềm."],
  ["Phân loại lỗi", "Câu đủ dữ liệu sẽ pass. Câu thiếu đáp án, trùng nội dung hoặc thiếu lựa chọn cần kiểm tra."],
  ["Giáo viên duyệt", "Sửa câu mơ hồ, gắn ảnh, xác nhận đáp án trước khi lưu."],
];

type FileKind = "none" | "text" | "needs-ocr" | "needs-conversion" | "unsupported";
type ImportTab = "source" | "results" | "review";
type ApprovalSummary = {
  approved: number;
  alreadyApproved: number;
  skipped: number;
  rejected: number;
};

export function TeacherCreateExam() {
  const auth = useRequiredAuth("teacher");
  const navigate = useNavigate();
  const [stage, setStage] = useState<"setup" | "preview">("setup");
  const [activeTab, setActiveTab] = useState<ImportTab>("source");
  const [fileState, setFileState] = useState(emptyFileState);
  const [fileKind, setFileKind] = useState<FileKind>("none");
  const [selectedFile, setSelectedFile] = useState<File | undefined>();
  const [importBatchID, setImportBatchID] = useState<number | undefined>();
  const [isParsing, setIsParsing] = useState(false);
  const [rawText, setRawText] = useState("");
  const [checkedText, setCheckedText] = useState("");
  const [reviewResult, setReviewResult] = useState<ParseResult>({ questions: [], summary: { total: 0, passed: 0, review: 0, failed: 0, averageConfidence: 0 } });
  const [isTextDirty, setIsTextDirty] = useState(false);
  const [extractNote, setExtractNote] = useState("");
  const [result, setResult] = useState("Chọn file đề thi để bắt đầu.");
  const [isApproving, setIsApproving] = useState(false);
  const [approvalSummary, setApprovalSummary] = useState<ApprovalSummary | undefined>();
  const classQuery = useQuery({ queryKey: ["teacher-classes"], queryFn: getTeacherClasses });
  const parseResult = useMemo(() => parseExamText(checkedText), [checkedText]);
  const pipelineStep = getPipelineStep(stage, fileKind, isParsing, checkedText, isTextDirty);
  const activeResult = reviewResult.questions.length ? reviewResult : parseResult;

  if (!auth) return <Navigate to="/" replace />;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedFile) {
      setResult("Cần chọn file đề thi trước khi sang bước xử lý.");
      return;
    }

    setIsParsing(true);
    setResult("Server đang nhận file và xử lý parser...");
    try {
      const response = await parseTeacherImport(selectedFile);
      const nextText = response.extract.text || "";
      setImportBatchID(response.importBatchId);
      setRawText(nextText);
      setCheckedText(nextText);
      setReviewResult({ questions: response.questions, summary: response.summary });
      setIsTextDirty(false);
      setFileKind(
        response.extract.status === "unsupported"
          ? "unsupported"
          : nextText
            ? "text"
            : response.extract.needsConversion
              ? "needs-conversion"
              : response.extract.needsOcr
                ? "needs-ocr"
                : "text",
      );
      setExtractNote([
        response.extract.documentTitle ? `Tiêu đề: ${response.extract.documentTitle}` : "",
        response.extract.imageCount ? `Ảnh nhúng: ${response.extract.imageCount}` : "",
        response.extract.pageEstimate ? `Trang ước tính: ${response.extract.pageEstimate}` : "",
        response.extract.warning,
      ].filter(Boolean).join(" | "));
      setStage("preview");
      setActiveTab(nextText ? "results" : "source");
      setResult(`${response.message || `Server đã tách ${response.summary.total} câu.`} Đã lưu batch #${response.importBatchId} vào database.`);
    } catch (error) {
      setResult(error instanceof Error ? error.message : "Không xử lý được file trên server.");
    } finally {
      setIsParsing(false);
    }
  }

  function receiveSourceFile(file?: File) {
    if (!file) {
      setFileState(emptyFileState);
      setFileKind("none");
      setSelectedFile(undefined);
      setImportBatchID(undefined);
      setRawText("");
      setCheckedText("");
      setReviewResult({ questions: [], summary: { total: 0, passed: 0, review: 0, failed: 0, averageConfidence: 0 } });
      setIsTextDirty(false);
      setExtractNote("");
      setResult("Chọn file đề thi để bắt đầu.");
      return;
    }

    setFileState(`${file.name} - ${(file.size / 1024).toFixed(1)} KB`);
    setSelectedFile(file);
    setImportBatchID(undefined);
    setFileKind("none");
    setRawText("");
    setCheckedText("");
    setReviewResult({ questions: [], summary: { total: 0, passed: 0, review: 0, failed: 0, averageConfidence: 0 } });
    setIsTextDirty(false);
    setExtractNote("");
    setResult("Đã nhận file. Bấm Tiếp theo để server xử lý.");
  }

  function updateRawText(value: string) {
    setRawText(value);
    setIsTextDirty(value !== checkedText);
  }

  function rerunLocalCheck() {
    setCheckedText(rawText);
    setReviewResult(mergeImportItemIDs(parseExamText(rawText), activeResult));
    setIsTextDirty(false);
    setActiveTab("results");
  }

  function restoreExtractedText() {
    setRawText(checkedText);
    setIsTextDirty(false);
  }

  async function saveReviewQuestion(index: number, nextQuestion: ParsedQuestion) {
    const nextResult = recheckParsedQuestions(activeResult.questions.map((question, questionIndex) => (
      questionIndex === index ? nextQuestion : question
    )));
    const savedQuestion = nextResult.questions[index];
    if (!importBatchID || !savedQuestion.importItemId) {
      setResult("Câu này chưa có item trong database. Hãy chạy lại import từ file nếu cần lưu.");
      throw new Error("missing import item id");
    }
    await saveTeacherImportItem(importBatchID, savedQuestion);
    setReviewResult(nextResult);
    setResult(`Đã lưu Câu ${savedQuestion.sourceOrder} vào batch #${importBatchID}.`);
  }

  async function approvePassQuestions() {
    const passQuestions = activeResult.questions.filter((question) => question.status === "pass");
    if (!importBatchID) {
      setResult("Batch này chưa có trong database, chưa thể lưu vào ngân hàng câu hỏi.");
      return;
    }
    if (passQuestions.length === 0) {
      setResult("Chưa có câu pass để lưu. Cần sửa ít nhất một câu trước khi tiếp tục.");
      return;
    }

    setIsApproving(true);
    setResult("Đang lưu các câu pass vào ngân hàng câu hỏi...");
    try {
      const response = await approveTeacherImportPassItems(importBatchID);
      setReviewResult(resultFromQuestions(passQuestions));
      setActiveTab("results");
      setApprovalSummary({
        approved: response.approved,
        alreadyApproved: response.alreadyApproved,
        skipped: response.skipped,
        rejected: response.rejected,
      });
      setResult(
        `Đã lưu ${response.approved} câu pass vào ngân hàng câu hỏi. ` +
        `${response.alreadyApproved ? `${response.alreadyApproved} câu đã lưu trước đó. ` : ""}` +
        `${response.skipped} câu chưa pass được giữ lại trong batch để sửa sau.`,
      );
    } catch (error) {
      setResult(error instanceof Error ? error.message : "Không lưu được các câu pass vào ngân hàng câu hỏi.");
    } finally {
      setIsApproving(false);
    }
  }

  return (
    <PageShell backTo="/teacher">
      <main className={`create-page ${stage === "preview" ? "create-page-compact" : ""}`}>
        {stage === "setup" && (
          <section className="create-hero">
            <div>
              <p className="eyebrow">Tạo bài kiểm tra</p>
              <h1>Chuẩn bị file đề thi</h1>
              <p className="lead">Nhập thông tin bài kiểm tra và chọn file. Bước xử lý câu hỏi sẽ nằm ở màn tiếp theo.</p>
            </div>
          </section>
        )}

        {stage === "setup" ? (
          <section className="create-layout">
            <form className="create-form" onSubmit={submit}>
              <label htmlFor="examTitle">Tên bài kiểm tra</label>
              <input id="examTitle" placeholder="VD: Cơ sở dữ liệu - Kiểm tra 15 phút" required />

              <label htmlFor="examMode">Loại bài</label>
              <select id="examMode">
                <option>Thi thử</option>
                <option>Thi chính thức</option>
              </select>

              <label htmlFor="targetClass">Lớp áp dụng</label>
              <select id="targetClass" required>
                {classQuery.data && classQuery.data.length > 0 ? (
                  classQuery.data.map((classItem) => (
                    <option value={classItem.id} key={classItem.id}>
                      {classItem.classCode} - {classItem.className}
                    </option>
                  ))
                ) : (
                  <option value="">{classQuery.isLoading ? "Đang tải lớp..." : "Chưa có lớp trong database"}</option>
                )}
              </select>

              <label htmlFor="startTime">Thời gian mở bài</label>
              <input id="startTime" type="datetime-local" />

              <label htmlFor="duration">Thời lượng</label>
              <input id="duration" type="number" min="5" defaultValue="45" />

              <div className="label-with-help">
                <label htmlFor="examFile">File đề thi</label>
                <span className="help-tip" tabIndex={0} aria-label="Luật parser">
                  ?
                  <span className="help-popover">
                    Hệ thống tách câu hỏi bằng luật mềm trước: nhận Câu 1, 1., 1/, lựa chọn A., A), hoặc tự đoán lựa chọn không nhãn. AI chỉ nên là fallback sau này.
                  </span>
                </span>
              </div>
              <input 
                id="examFile"
                type="file"
                accept=".doc,.docx,.pdf,.txt,.rtf,.csv"
                onChange={(event) => receiveSourceFile(event.target.files?.[0])}
              />
              <p className="form-note">
                TXT/CSV/DOCX/PDF sẽ được gửi lên server để xử lý. DOC cũ sẽ tự convert nếu server có LibreOffice.
              </p>
              <div className="format-list" aria-label="Dinh dang ho tro">
                {acceptedFormats.map((format) => <span key={format}>{format}</span>)}
              </div>

              <button className="primary-btn" type="submit" disabled={isParsing}>
                {isParsing ? "Đang xử lý..." : "Tiếp theo"}
              </button>
            </form>

            <PipelinePanel fileState={fileState} result={result} activeStep={pipelineStep} />
          </section>
        ) : (
          <section className="processing-layout processing-layout-compact">
            <div className="processing-workspace">
              <div className="processing-toolbar">
                <div>
                  <p className="eyebrow">Bước 2</p>
                  <h2>Kiểm tra câu hỏi{importBatchID ? ` - Batch #${importBatchID}` : ""}</h2>
                </div>
                <button className="ghost-btn" type="button" onClick={() => setStage("setup")}>Quay lại thông tin đề</button>
              </div>

              {fileKind === "needs-ocr" && (
                <div className="parser-empty">
                  Đã nhận {fileState}. File này cần OCR trước khi parser local có dữ liệu để preview.
                </div>
              )}

              {fileKind === "needs-conversion" && (
                <div className="parser-empty">
                  Đã nhận {fileState}. Đây là DOC cũ hoặc file cần converter server trước khi tách text; hệ thống vẫn đã lưu batch và metadata ảnh.
                </div>
              )}

              {fileKind === "unsupported" && (
                <div className="parser-empty">
                  Đã nhận {fileState}, nhưng server chưa hỗ trợ định dạng này ở bước import đầu tiên.
                </div>
              )}

              {extractNote && <p className="compact-meta">{extractNote}</p>}

              <div className="import-tabs" role="tablist" aria-label="Các tab kiểm tra import">
                <button className={activeTab === "source" ? "active" : ""} type="button" onClick={() => setActiveTab("source")}>Nguồn đề</button>
                <button className={activeTab === "results" ? "active" : ""} type="button" onClick={() => setActiveTab("results")}>Kết quả tách</button>
                <button className={activeTab === "review" ? "active" : ""} type="button" onClick={() => setActiveTab("review")}>Cần kiểm tra</button>
              </div>

              {activeTab === "source" && (
                <section className="source-review-panel">
                  <div className="source-review-head">
                    <div>
                      <h3>Soát nguồn đã tách</h3>
                      <p className="form-note">Sửa text ở đây nếu cần thêm đáp án, tách lại câu bị dính, hoặc xoá dòng thừa.</p>
                    </div>
                    {isTextDirty && <span className="dirty-badge">Có thay đổi chưa kiểm tra lại</span>}
                  </div>
                  <textarea
                    id="rawExamText"
                    value={rawText}
                    onChange={(event) => updateRawText(event.target.value)}
                    placeholder="Text sau khi tách sẽ nằm ở đây. Sửa xong bấm Chạy lại kiểm tra."
                    rows={16}
                  />
                  <div className="source-actions">
                    <button className="primary-btn" type="button" onClick={rerunLocalCheck} disabled={!rawText.trim()}>Chạy lại kiểm tra</button>
                    <button className="ghost-btn" type="button" onClick={restoreExtractedText} disabled={!isTextDirty}>Khôi phục bản đang kiểm tra</button>
                  </div>
                </section>
              )}

              {activeTab === "results" && (
                <QuestionImportPreview result={activeResult} onQuestionSave={saveReviewQuestion} />
              )}

              {activeTab === "review" && (
                <QuestionImportPreview result={activeResult} mode="needs-review" onQuestionSave={saveReviewQuestion} />
              )}

              <div className="approval-actions">
                <div>
                  <strong>Tiếp tục với câu đã pass</strong>
                  <span>Câu cần kiểm tra hoặc lỗi sẽ không đi vào ngân hàng câu hỏi ở bước này.</span>
                </div>
                <button className="primary-btn" type="button" onClick={approvePassQuestions} disabled={isApproving || activeResult.summary.passed === 0}>
                  {isApproving ? "Đang lưu..." : `Lưu ${activeResult.summary.passed} câu pass`}
                </button>
              </div>
            </div>

            <p className="compact-result">{result}</p>
          </section>
        )}

        {approvalSummary && (
          <ApprovalResultModal
            summary={approvalSummary}
            onConfirm={() => navigate("/teacher", { replace: true })}
          />
        )}
      </main>
    </PageShell>
  );
}

function ApprovalResultModal({ summary, onConfirm }: { summary: ApprovalSummary; onConfirm: () => void }) {
  return (
    <div className="approval-modal-backdrop" role="presentation">
      <section className="approval-modal" role="dialog" aria-modal="true" aria-label="Kết quả lưu câu hỏi">
        <p className="eyebrow">Đã lưu vào database</p>
        <h2>Hoàn tất bước duyệt câu pass</h2>
        <p>
          Hệ thống đã đưa các câu hợp lệ vào ngân hàng câu hỏi. Những câu chưa pass vẫn được giữ trong batch import để sửa sau.
        </p>
        <div className="approval-modal-stats">
          <span><strong>{summary.approved}</strong>Câu mới đã lưu</span>
          <span><strong>{summary.alreadyApproved}</strong>Câu đã lưu trước đó</span>
          <span><strong>{summary.skipped}</strong>Câu giữ lại để sửa</span>
          <span><strong>{summary.rejected}</strong>Câu lỗi</span>
        </div>
        <button className="primary-btn" type="button" onClick={onConfirm}>
          Về dashboard giáo viên
        </button>
      </section>
    </div>
  );
}

function getPipelineStep(stage: "setup" | "preview", fileKind: FileKind, isParsing: boolean, checkedText: string, isTextDirty: boolean) {
  if (isParsing) return 1;
  if (stage === "setup") return 0;
  if (fileKind === "needs-ocr" || fileKind === "needs-conversion" || fileKind === "unsupported") return 1;
  if (isTextDirty) return 2;
  if (checkedText.trim()) return 4;
  return 2;
}

function mergeImportItemIDs(next: ParseResult, previous: ParseResult): ParseResult {
  const previousByOrder = new Map(previous.questions.map((question) => [question.sourceOrder, question.importItemId]));
  return resultFromQuestions(next.questions.map((question, index) => ({
    ...question,
    importItemId: question.importItemId || previousByOrder.get(question.sourceOrder) || previous.questions[index]?.importItemId,
  })));
}

function resultFromQuestions(questions: ParsedQuestion[]): ParseResult {
  const total = questions.length;
  const passed = questions.filter((question) => question.status === "pass").length;
  const review = questions.filter((question) => question.status === "review").length;
  const failed = questions.filter((question) => question.status === "fail").length;
  const averageConfidence = total
    ? Math.round(questions.reduce((sum, question) => sum + question.confidence, 0) / total)
    : 0;
  return { questions, summary: { total, passed, review, failed, averageConfidence } };
}

function PipelinePanel({ fileState, result, activeStep }: { fileState: string; result: string; activeStep: number }) {
  return (
    <aside className="ai-pipeline">
      <p className="eyebrow">Local format pipeline</p>
      <h2>Tách câu hỏi không cần AI</h2>
      {pipelineSteps.map(([title, text], index) => (
        <div className={`pipeline-step ${index === activeStep ? "active" : ""} ${index < activeStep ? "done" : ""}`} key={title}>
          <strong>{index + 1}. {title}</strong>
          <span>{index === 0 ? fileState : text}</span>
        </div>
      ))}

      <div className="pipeline-note">
        <strong>Luật phân loại</strong>
        <span>Thiếu đáp án đúng không được pass. Câu trùng nội dung, thiếu lựa chọn hoặc thiếu dữ liệu ảnh sẽ được đẩy sang cần kiểm tra.</span>
      </div>

      <div className="pipeline-result">{result}</div>
    </aside>
  );
}
