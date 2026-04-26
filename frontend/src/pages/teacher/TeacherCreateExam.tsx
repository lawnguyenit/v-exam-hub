import type { FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import {
  approveTeacherImportPassItems,
  createTeacherExam,
  createTeacherImportItem,
  deleteTeacherQuestionBank,
  deleteTeacherImportItem,
  getTeacherExam,
  getTeacherClasses,
  getTeacherQuestionBank,
  parseTeacherImport,
  saveTeacherImportItem,
} from "../../api";
import type { ImportDuplicateCandidate, QuestionBankItem } from "../../api";
import { QuestionImportPreview } from "../../features/question-import/QuestionImportPreview";
import { parseExamText, recheckParsedQuestions } from "../../features/question-import/ruleParser";
import type { ParsedQuestion, ParseResult } from "../../features/question-import/ruleParser";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

const emptyFileState = "Chưa có file";

type ImportGuideID = "txt" | "csv" | "docx" | "doc" | "pdfText" | "pdfScan";

type ImportGuide = {
  id: ImportGuideID;
  label: string;
  status?: "demo";
  title: string;
  summary: string;
  rules: string[];
  example: string;
};

const importGuides: ImportGuide[] = [
  {
    id: "txt",
    label: "TXT",
    title: "TXT thuần",
    summary: "Phù hợp khi giáo viên copy đề từ Word hoặc hệ thống cũ rồi lưu thành text.",
    rules: [
      "Mỗi câu nên bắt đầu bằng Câu 1:, Câu 1. hoặc 1.",
      "Lựa chọn nên dùng A., B., C., D. hoặc A), B), C), D).",
      "Đáp án đặt ngay sau câu bằng Đáp án: A hoặc Answer: A.",
    ],
    example: `Câu 1: Thiết bị nào dùng để lưu trữ dữ liệu lâu dài?
A. RAM
B. Ổ cứng
C. CPU
D. Màn hình
Đáp án: B`,
  },
  {
    id: "csv",
    label: "CSV",
    title: "CSV bảng câu hỏi",
    summary: "Dùng khi đã có danh sách câu hỏi dạng bảng và muốn import nhanh, ít lỗi.",
    rules: [
      "Dòng đầu nên có tên cột: question,A,B,C,D,answer.",
      "Mỗi dòng là một câu; đáp án chỉ ghi A, B, C hoặc D.",
      "Nếu nội dung có dấu phẩy, hãy đặt ô đó trong dấu nháy kép.",
    ],
    example: `question,A,B,C,D,answer
"2 + 2 bằng bao nhiêu?","3","4","5","6",B`,
  },
  {
    id: "docx",
    label: "DOCX",
    title: "Word DOCX",
    summary: "Định dạng nên ưu tiên cho đề có tiếng Việt, bảng hoặc hình ảnh.",
    rules: [
      "Giữ câu hỏi, lựa chọn và đáp án gần nhau trong cùng một cụm nội dung.",
      "Hình ảnh nên nằm ngay dưới câu hỏi hoặc ngay dưới lựa chọn liên quan.",
      "Có thể đánh dấu đáp án bằng dòng Đáp án: A hoặc tô đỏ lựa chọn đúng nếu đề gốc dùng kiểu đó.",
    ],
    example: `Câu 1: Quan sát hình sau và chọn phát biểu đúng.
[Hình minh hoạ nằm ngay dưới câu]
A. Phát biểu thứ nhất
B. Phát biểu thứ hai
C. Phát biểu thứ ba
D. Phát biểu thứ tư
Đáp án: C`,
  },
  {
    id: "doc",
    label: "DOC cũ",
    title: "Word DOC cũ",
    summary: "Hệ thống sẽ cố chuyển DOC cũ sang DOCX trước khi tách câu.",
    rules: [
      "Nên dùng khi chỉ còn file Word cũ; nếu có thể, hãy lưu lại thành DOCX để ổn định hơn.",
      "Không dùng textbox hoặc layout nhiều cột cho phần câu hỏi chính.",
      "Bảng và hình có thể cần kiểm tra lại sau khi import.",
    ],
    example: `1. Câu hỏi có thể viết theo số thứ tự
A. Lựa chọn A
B. Lựa chọn B
C. Lựa chọn C
D. Lựa chọn D
Đáp án: D`,
  },
  {
    id: "pdfText",
    label: "PDF text",
    status: "demo",
    title: "PDF có text",
    summary: "Đang ở mức demo. PDF xuất từ Word có thể tách được chữ, nhưng bảng, hình và bố cục nhiều cột vẫn cần duyệt kỹ.",
    rules: [
      "PDF nên có text thật, không phải ảnh scan.",
      "Tránh đề chia nhiều cột vì thứ tự câu có thể bị đảo khi tách text.",
      "Câu có bảng hoặc hình cần được giáo viên kiểm tra lại ở bước duyệt.",
    ],
    example: `Câu 1. Nội dung câu hỏi
A. Lựa chọn A
B. Lựa chọn B
C. Lựa chọn C
D. Lựa chọn D
Đáp án: A`,
  },
  {
    id: "pdfScan",
    label: "PDF scan",
    status: "demo",
    title: "PDF scan",
    summary: "Đang ở mức demo. PDF scan cần OCR nên kết quả phụ thuộc mạnh vào chất lượng ảnh, độ thẳng trang và bố cục đề.",
    rules: [
      "Ảnh scan càng rõ, thẳng trang, ít nhiễu thì kết quả càng ổn.",
      "Câu có hình, bảng hoặc công thức nên kiểm tra thủ công sau khi OCR.",
      "Nếu OCR ra thiếu chữ hoặc sai thứ tự, nên sửa ở bước nguồn đề trước khi lưu.",
    ],
    example: `Sau OCR, hệ thống cần nhìn thấy dạng gần như:
Câu 1: ...
A. ...
B. ...
C. ...
D. ...
Đáp án: ...`,
  },
];

type FileKind = "none" | "text" | "needs-ocr" | "needs-conversion" | "unsupported";
type ImportTab = "source" | "results" | "review";
type CreateStage = "choose" | "import" | "preview" | "compose";
type CreateSource = "import" | "bank";

type ApprovalSummary = {
  approved: number;
  alreadyApproved: number;
  skipped: number;
  rejected: number;
  questionIds: number[];
};

type ExamForm = {
  title: string;
  description: string;
  examMode: "practice" | "official" | "attendance";
  classId: string;
  startTime: string;
  durationMinutes: string;
  maxAttemptsPerStudent: string;
  questionSourceId: string;
  questionCount: string;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  showResultImmediately: boolean;
  allowReview: boolean;
};

const defaultExamForm: ExamForm = {
  title: "",
  description: "",
  examMode: "practice",
  classId: "",
  startTime: "",
  durationMinutes: "45",
  maxAttemptsPerStudent: "1",
  questionSourceId: "",
  questionCount: "",
  shuffleQuestions: false,
  shuffleOptions: false,
  showResultImmediately: false,
  allowReview: true,
};

export function TeacherCreateExam() {
  const auth = useRequiredAuth("teacher");
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const editExamID = searchParams.get("exam") || "";
  const initialSourceID = searchParams.get("source") || "";
  const queryClient = useQueryClient();
  const [stage, setStage] = useState<CreateStage>(editExamID ? "compose" : "choose");
  const [source, setSource] = useState<CreateSource>(editExamID ? "bank" : "import");
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
  const [result, setResult] = useState("Chọn cách tạo bài kiểm tra.");
  const [isApproving, setIsApproving] = useState(false);
  const [approvalSummary, setApprovalSummary] = useState<ApprovalSummary | undefined>();
  const [examForm, setExamForm] = useState<ExamForm>(defaultExamForm);
  const [isCreatingExam, setIsCreatingExam] = useState(false);
  const [deletingBankID, setDeletingBankID] = useState<number | undefined>();
  const [pendingDeleteBank, setPendingDeleteBank] = useState<QuestionBankItem | null>(null);
  const [duplicateCandidates, setDuplicateCandidates] = useState<ImportDuplicateCandidate[]>([]);
  const [isDuplicateModalOpen, setIsDuplicateModalOpen] = useState(false);
  const [mergeTargetBatchID, setMergeTargetBatchID] = useState<number | undefined>();
  const [activeImportGuideID, setActiveImportGuideID] = useState<ImportGuideID | undefined>();

  const classQuery = useQuery({ queryKey: ["teacher-classes"], queryFn: getTeacherClasses });
  const questionBankQuery = useQuery({
    queryKey: ["teacher-question-bank", auth?.account],
    queryFn: () => getTeacherQuestionBank(auth?.account),
    enabled: (stage === "compose" || Boolean(initialSourceID)) && Boolean(auth?.account),
  });
  const editExamQuery = useQuery({
    queryKey: ["teacher-exam", editExamID],
    queryFn: () => getTeacherExam(editExamID),
    enabled: Boolean(editExamID),
  });
  const parseResult = useMemo(() => parseExamText(checkedText), [checkedText]);
  const activeResult = reviewResult.questions.length ? reviewResult : parseResult;
  const questionBank = questionBankQuery.data || [];

  useEffect(() => {
    const exam = editExamQuery.data;
    if (!exam) return;
    setSource("bank");
    setStage("compose");
    setExamForm({
      title: exam.title,
      description: exam.description || "",
      examMode: exam.examMode || "practice",
      classId: exam.classId ? String(exam.classId) : "",
      startTime: exam.startValue || "",
      durationMinutes: String(exam.durationMinutes || 45),
      maxAttemptsPerStudent: String(exam.maxAttemptsPerStudent ?? 1),
      questionSourceId: exam.questionSourceId ? String(exam.questionSourceId) : "",
      questionCount: String(exam.questionCount || ""),
      shuffleQuestions: Boolean(exam.shuffleQuestions),
      shuffleOptions: Boolean(exam.shuffleOptions),
      showResultImmediately: Boolean(exam.showResultImmediately),
      allowReview: Boolean(exam.allowReview),
    });
    setResult(exam.canEdit ? "Đang sửa cấu hình bài kiểm tra." : "Bài đã có lượt làm: nguồn đề cương bị khóa, cấu hình khác sẽ áp dụng cho lượt làm sau.");
  }, [editExamQuery.data]);

  useEffect(() => {
    if (!initialSourceID || editExamID || examForm.questionSourceId || questionBank.length === 0) return;
    const sourceItem = questionBank.find((bank) => String(bank.id) === initialSourceID);
    if (!sourceItem) return;
    setSource("bank");
    setStage("compose");
    setExamForm((current) => ({
      ...current,
      questionSourceId: String(sourceItem.id),
      questionCount: String(Math.min(40, sourceItem.questionCount)),
    }));
  }, [editExamID, examForm.questionSourceId, initialSourceID, questionBank]);

  if (!auth) return <Navigate to="/" replace />;
  const account = auth.account;
  const activeImportGuide = importGuides.find((guide) => guide.id === activeImportGuideID);

  function chooseSource(nextSource: CreateSource) {
    setSource(nextSource);
    setStage(nextSource === "import" ? "import" : "compose");
    setResult(nextSource === "import" ? "Chọn file đề cương để đưa câu hỏi vào ngân hàng trước." : "Chọn câu hỏi có sẵn trong ngân hàng để tạo bài kiểm tra.");
    if (nextSource === "bank") {
      setExamForm((current) => ({ ...current, questionSourceId: "", questionCount: "" }));
    }
  }

  async function submitImport(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedFile) {
      setResult("Cần chọn file đề thi trước khi sang bước xử lý.");
      return;
    }

    setIsParsing(true);
    setResult("Server đang nhận file và xử lý parser...");
    try {
      const response = await parseTeacherImport(selectedFile, account);
      const nextText = response.extract.text || "";
      setImportBatchID(response.importBatchId);
      setRawText(nextText);
      setCheckedText(nextText);
      setReviewResult({ importBatchId: response.importBatchId, questions: response.questions, summary: response.summary });
      const candidates = response.duplicateCandidates || [];
      setDuplicateCandidates(candidates);
      setMergeTargetBatchID(candidates[0]?.batchId);
      setIsDuplicateModalOpen(candidates.length > 0);
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
      setDuplicateCandidates([]);
      setIsDuplicateModalOpen(false);
      setMergeTargetBatchID(undefined);
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
    setDuplicateCandidates([]);
    setIsDuplicateModalOpen(false);
    setMergeTargetBatchID(undefined);
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
    nextResult.importBatchId = activeResult.importBatchId;
    const savedQuestion = nextResult.questions[index];
    if (!importBatchID || !savedQuestion.importItemId) {
      setResult("Câu này chưa có item trong database. Hãy chạy lại import từ file nếu cần lưu.");
      throw new Error("missing import item id");
    }
    await saveTeacherImportItem(importBatchID, savedQuestion);
    setReviewResult(nextResult);
    setResult(`Đã lưu Câu ${savedQuestion.sourceOrder} vào batch #${importBatchID}.`);
  }

  async function createReviewQuestion(nextQuestion: ParsedQuestion) {
    const nextResult = recheckParsedQuestions([...activeResult.questions, nextQuestion]);
    nextResult.importBatchId = activeResult.importBatchId;
    const savedQuestion = nextResult.questions[nextResult.questions.length - 1];
    if (!importBatchID) {
      setResult("Batch nay chua co trong database. Hay import file truoc khi them cau thu cong.");
      throw new Error("missing import batch id");
    }
    const createdQuestion = await createTeacherImportItem(importBatchID, savedQuestion);
    setReviewResult(resultFromQuestions([...activeResult.questions, createdQuestion], activeResult.importBatchId));
    setResult(`Da them Cau ${createdQuestion.sourceOrder} vao batch #${importBatchID}.`);
  }

  async function deleteReviewQuestion(index: number, question: ParsedQuestion) {
    if (importBatchID && question.importItemId) {
      await deleteTeacherImportItem(importBatchID, question.importItemId);
    }
    const nextQuestions = activeResult.questions.filter((_, questionIndex) => questionIndex !== index);
    setReviewResult(resultFromQuestions(nextQuestions, activeResult.importBatchId));
    setResult(`Đã xoá Câu ${question.sourceOrder} khỏi danh sách duyệt.`);
  }

  async function approvePassQuestions(targetBatchID?: number | null) {
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
      const requestedTargetBatchID = targetBatchID === null ? undefined : targetBatchID ?? mergeTargetBatchID;
      const response = await approveTeacherImportPassItems(importBatchID, requestedTargetBatchID);
      await queryClient.invalidateQueries({ queryKey: ["teacher-question-bank"] });
      setReviewResult(resultFromQuestions(passQuestions, activeResult.importBatchId));
      setActiveTab("results");
      const sourceID = response.targetBatchId || response.importBatchId || importBatchID;
      setApprovalSummary({
        approved: response.approved,
        alreadyApproved: response.alreadyApproved,
        skipped: response.skipped,
        rejected: response.rejected,
        questionIds: response.questionIds || [],
      });
      setExamForm((current) => ({
        ...current,
        questionSourceId: String(sourceID),
        questionCount: String(response.questionCount || response.questionIds?.length || response.approved + response.alreadyApproved),
      }));
      setStage("compose");
      if (requestedTargetBatchID) {
        setResult(`Đã thêm ${response.approved} câu mới vào đề cương đã có và bỏ qua ${response.skipped} câu trùng.`);
        return;
      }
      setResult(`Đã đưa ${response.approved} câu mới vào ngân hàng. Tiếp tục cấu hình bài kiểm tra.`);
    } catch (error) {
      setResult(error instanceof Error ? error.message : "Không lưu được các câu pass vào ngân hàng câu hỏi.");
    } finally {
      setIsApproving(false);
    }
  }

  async function submitExam(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!formSource(examForm)) {
      setResult("Cần chọn một bộ đề cương để tạo bài kiểm tra.");
      return;
    }
    if (Number(examForm.questionCount) <= 0) {
      setResult("Cần nhập số câu lấy từ bộ đề cương.");
      return;
    }
    setIsCreatingExam(true);
    setResult("Đang tạo bài kiểm tra từ ngân hàng câu hỏi...");
    try {
      const created = await createTeacherExam({
        examId: editExamID || undefined,
        createdBy: account,
        title: examForm.title,
        description: examForm.description,
        examMode: examForm.examMode,
        classId: Number(examForm.classId),
        startTime: examForm.startTime,
        durationMinutes: Number(examForm.durationMinutes),
        maxAttemptsPerStudent: Number(examForm.maxAttemptsPerStudent),
        shuffleQuestions: examForm.shuffleQuestions,
        shuffleOptions: examForm.shuffleOptions,
        showResultImmediately: examForm.showResultImmediately,
        allowReview: examForm.allowReview,
        questionIds: [],
        questionSourceId: Number(examForm.questionSourceId),
        questionCount: Number(examForm.questionCount),
      });
      setResult(`${editExamID ? "Đã cập nhật" : "Đã tạo"} bài kiểm tra #${created.id} với ${created.questionCount} câu. Trạng thái: ${created.status}.`);
      navigate("/teacher", { replace: true });
    } catch (error) {
      setResult(error instanceof Error ? error.message : "Không tạo được bài kiểm tra.");
    } finally {
      setIsCreatingExam(false);
    }
  }

  async function deleteQuestionBankSource(bank: QuestionBankItem) {
    if (!auth) return;
    setDeletingBankID(bank.id);
    try {
      const deleted = await deleteTeacherQuestionBank(bank.id, auth.account);
      await queryClient.invalidateQueries({ queryKey: ["teacher-question-bank"] });
      if (examForm.questionSourceId === String(bank.id)) {
        setExamForm({ ...examForm, questionSourceId: "", questionCount: "" });
      }
      setResult(`Đã xóa bộ đề cương: ${deleted.deletedQuestions} câu được xóa, ${deleted.archivedQuestions} câu đã archive.`);
      setPendingDeleteBank(null);
    } catch (error) {
      setResult(error instanceof Error ? error.message : "Không xóa được bộ đề cương.");
    } finally {
      setDeletingBankID(undefined);
    }
  }

  async function chooseDuplicateMerge(candidate: ImportDuplicateCandidate) {
    setMergeTargetBatchID(candidate.batchId);
    setIsDuplicateModalOpen(false);
    await approvePassQuestions(candidate.batchId);
  }

  async function chooseDuplicateNewSource() {
    setMergeTargetBatchID(undefined);
    setIsDuplicateModalOpen(false);
    await approvePassQuestions(null);
  }

  return (
    <PageShell backTo="/teacher">
      <main className={`create-page ${stage !== "choose" ? "create-page-compact" : ""}`}>
        <section className="create-hero">
          <div>
            <p className="eyebrow">Tạo bài kiểm tra</p>
            <h1>{editExamID ? "Sửa cấu hình bài kiểm tra" : stage === "choose" ? "Chọn cách tạo bài" : stage === "compose" ? "Cấu hình bài kiểm tra" : "Import câu hỏi vào ngân hàng"}</h1>
            <p className="lead">
              {stage === "choose"
                  ? "Tạo bài mới từ file đề cương hoặc dùng câu hỏi đã có trong ngân hàng."
                  : stage === "compose"
                    ? "Chọn bộ đề cương, số câu, lớp áp dụng và thời gian mở bài."
                    : "File đề cương sẽ được tách câu, duyệt pass, rồi chuyển sang bước tạo bài."}
            </p>
          </div>
        </section>

        {stage === "choose" && (
          <section className="create-choice-grid">
            <button className="create-choice-card" type="button" onClick={() => chooseSource("import")}>
              <span className="eyebrow">Lựa chọn 1</span>
              <strong>Import ngân hàng câu hỏi rồi tạo bài</strong>
              <p>Dùng khi có file Word/PDF/TXT/CSV. Hệ thống tách câu, giáo viên duyệt, sau đó lấy câu pass để tạo bài.</p>
            </button>
            <button className="create-choice-card" type="button" onClick={() => chooseSource("bank")}>
              <span className="eyebrow">Lựa chọn 2</span>
              <strong>Tạo bài từ ngân hàng đã có</strong>
              <p>Dùng lại câu hỏi đã import trước đó. Không cần upload file mới.</p>
            </button>
          </section>
        )}

        {stage === "import" && (
          <section className={`create-layout import-help-layout ${activeImportGuide ? "import-help-layout-open" : ""}`}>
            <form className="create-form" onSubmit={submitImport}>
              <div className="label-with-help">
                <label htmlFor="examFile">File đề cương</label>
                <span className="help-tip" tabIndex={0} aria-label="Luật parser">
                  ?
                  <span className="help-popover">
                    Hệ thống tách câu bằng luật mềm trước: nhận Câu 1, 1., 1/, lựa chọn A., A), hoặc tự đoán lựa chọn không nhãn.
                  </span>
                </span>
              </div>
              <input
                id="examFile"
                type="file"
                accept=".doc,.docx,.pdf,.txt,.rtf,.csv"
                onChange={(event) => receiveSourceFile(event.target.files?.[0])}
              />
              <p className="form-note">File đề cương sẽ được tách câu, duyệt pass, rồi chuyển tiếp sang bước tạo bài kiểm tra.</p>
              <div className="format-guide-head">
                <strong>Định dạng hỗ trợ</strong>
                <span>Ấn vào từng định dạng để xem hướng dẫn import đúng.</span>
              </div>
              <div className="format-list format-guide-list" aria-label="Định dạng hỗ trợ">
                {importGuides.map((guide) => (
                  <button
                    className={activeImportGuideID === guide.id ? "active" : ""}
                    type="button"
                    key={guide.id}
                    aria-pressed={activeImportGuideID === guide.id}
                    onClick={() => setActiveImportGuideID(guide.id)}
                  >
                    <strong>{guide.label}</strong>
                    <small>{guide.status === "demo" ? "Demo - xem hướng dẫn" : "Xem hướng dẫn"}</small>
                  </button>
                ))}
              </div>
              <button className="primary-btn" type="submit" disabled={isParsing}>
                {isParsing ? "Đang xử lý..." : "Tiếp theo"}
              </button>
              <button className="ghost-btn" type="button" onClick={() => setStage("choose")}>Đổi cách tạo</button>
            </form>
            {activeImportGuide && <ImportGuidePanel guide={activeImportGuide} onClose={() => setActiveImportGuideID(undefined)} />}
          </section>
        )}

        {stage === "preview" && (
          <section className="processing-layout processing-layout-compact">
            <div className="processing-workspace">
              <div className="processing-toolbar">
                <div>
                  <p className="eyebrow">Bước 2</p>
                  <h2>Kiểm tra câu hỏi{importBatchID ? ` - Batch #${importBatchID}` : ""}</h2>
                </div>
                <button className="ghost-btn" type="button" onClick={() => setStage("import")}>Quay lại chọn file</button>
              </div>

              {fileKind === "needs-ocr" && <div className="parser-empty">Đã nhận {fileState}. File này cần OCR trước khi parser local có dữ liệu để preview.</div>}
              {fileKind === "needs-conversion" && <div className="parser-empty">Đã nhận {fileState}. File này cần converter server trước khi tách text.</div>}
              {fileKind === "unsupported" && <div className="parser-empty">Đã nhận {fileState}, nhưng server chưa hỗ trợ định dạng này.</div>}
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
                      <p className="form-note">Sửa text nếu cần thêm đáp án, tách lại câu bị dính, hoặc xoá dòng thừa.</p>
                    </div>
                    {isTextDirty && <span className="dirty-badge">Có thay đổi chưa kiểm tra lại</span>}
                  </div>
                  <textarea value={rawText} onChange={(event) => updateRawText(event.target.value)} placeholder="Text sau khi tách sẽ nằm ở đây." rows={16} />
                  <div className="source-actions">
                    <button className="primary-btn" type="button" onClick={rerunLocalCheck} disabled={!rawText.trim()}>Chạy lại kiểm tra</button>
                    <button className="ghost-btn" type="button" onClick={restoreExtractedText} disabled={!isTextDirty}>Khôi phục bản đang kiểm tra</button>
                  </div>
                </section>
              )}

              {activeTab === "results" && <QuestionImportPreview result={activeResult} onQuestionSave={saveReviewQuestion} onQuestionCreate={createReviewQuestion} onQuestionDelete={deleteReviewQuestion} />}
              {activeTab === "review" && <QuestionImportPreview result={activeResult} mode="needs-review" onQuestionSave={saveReviewQuestion} onQuestionCreate={createReviewQuestion} onQuestionDelete={deleteReviewQuestion} />}

              <div className="approval-actions">
                <div>
                  <strong>Lưu câu pass rồi tạo bài</strong>
                  <span>Câu pass sẽ vào ngân hàng câu hỏi. Câu lỗi hoặc cần kiểm tra vẫn ở lại batch import.</span>
                </div>
                <button className="primary-btn" type="button" onClick={() => approvePassQuestions()} disabled={isApproving || activeResult.summary.passed === 0}>
                  {isApproving ? "Đang lưu..." : `Lưu ${activeResult.summary.passed} câu pass`}
                </button>
              </div>
            </div>
            <p className="compact-result">{result}</p>
          </section>
        )}

        {stage === "compose" && (
          <ExamComposer
            source={source}
            approvalSummary={approvalSummary}
            classes={classQuery.data || []}
            questionBank={questionBank}
            form={examForm}
            result={result}
            isLoadingQuestions={questionBankQuery.isLoading}
            isCreating={isCreatingExam}
            deletingBankID={deletingBankID}
            sourceLocked={Boolean(editExamID && editExamQuery.data?.canEdit === false)}
            onBack={() => editExamID ? navigate("/teacher") : setStage("choose")}
            onFormChange={setExamForm}
            onDeleteSource={setPendingDeleteBank}
            onSubmit={submitExam}
          />
        )}
        {isDuplicateModalOpen && duplicateCandidates.length > 0 && (
          <DuplicateImportModal
            candidates={duplicateCandidates}
            isApproving={isApproving}
            onMerge={chooseDuplicateMerge}
            onCreateNew={chooseDuplicateNewSource}
          />
        )}
        {pendingDeleteBank && (
          <div className="approval-modal-backdrop" role="presentation">
            <section className="teacher-confirm-modal" role="dialog" aria-modal="true" aria-label="Xác nhận xóa bộ đề cương">
              <div>
                <p className="eyebrow">Xóa bộ đề cương</p>
                <h2>{pendingDeleteBank.title}</h2>
              </div>
              <p>Các câu đã nằm trong bài thi cũ sẽ được lưu lịch sử và ẩn khỏi danh sách active. Những câu chưa được dùng sẽ bị xóa khỏi ngân hàng câu hỏi.</p>
              <div className="modal-actions">
                <button className="ghost-btn" type="button" onClick={() => setPendingDeleteBank(null)} disabled={deletingBankID === pendingDeleteBank.id}>Hủy</button>
                <button className="primary-btn danger" type="button" onClick={() => deleteQuestionBankSource(pendingDeleteBank)} disabled={deletingBankID === pendingDeleteBank.id}>
                  {deletingBankID === pendingDeleteBank.id ? "Đang xóa..." : "Xóa bộ đề cương"}
                </button>
              </div>
            </section>
          </div>
        )}
      </main>
    </PageShell>
  );
}

function ImportGuidePanel({ guide, onClose }: { guide: ImportGuide; onClose: () => void }) {
  return (
    <aside className="import-guide-panel" aria-label={`Hướng dẫn import ${guide.label}`}>
      <div className="import-guide-panel-head">
        <div>
          <p className="eyebrow">Hướng dẫn import</p>
          <h2>{guide.title}{guide.status === "demo" ? " - demo" : ""}</h2>
        </div>
        <button className="ghost-btn compact" type="button" onClick={onClose}>Thu gọn</button>
      </div>
      <p className="form-note">{guide.summary}</p>
      <div className="import-guide-rules">
        {guide.rules.map((rule) => (
          <span key={rule}>{rule}</span>
        ))}
      </div>
      <div className="import-guide-example">
        <strong>Mẫu nên dùng</strong>
        <pre>{guide.example}</pre>
      </div>
    </aside>
  );
}

function ExamComposer({
  source,
  approvalSummary,
  classes,
  questionBank,
  form,
  result,
  isLoadingQuestions,
  isCreating,
  deletingBankID,
  sourceLocked,
  onBack,
  onFormChange,
  onDeleteSource,
  onSubmit,
}: {
  source: CreateSource;
  approvalSummary?: ApprovalSummary;
  classes: Array<{ id: number; classCode: string; className: string }>;
  questionBank: QuestionBankItem[];
  form: ExamForm;
  result: string;
  isLoadingQuestions: boolean;
  isCreating: boolean;
  deletingBankID?: number;
  sourceLocked: boolean;
  onBack: () => void;
  onFormChange: (form: ExamForm) => void;
  onDeleteSource: (bank: QuestionBankItem) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const selectedSource = questionBank.find((bank) => String(bank.id) === form.questionSourceId);
  const requestedCount = Number(form.questionCount) || 0;
  const maxQuestions = selectedSource?.questionCount || 0;
  const submitLabel = sourceLocked ? `Cập nhật cấu hình với ${requestedCount || 0} câu` : `Lưu bài với ${requestedCount || 0} câu`;

  return (
    <section className="exam-compose-layout">
      <form className="create-form exam-compose-form" onSubmit={onSubmit}>
        <div className="compose-head">
          <div>
            <p className="eyebrow">{source === "import" ? "Sau import" : "Từ ngân hàng"}</p>
            <h2>Thông tin bài kiểm tra</h2>
          </div>
          <button className="ghost-btn" type="button" onClick={onBack}>Quay lại dashboard</button>
        </div>

        {approvalSummary && (
          <div className="compose-summary">
            <span><strong>{approvalSummary.approved}</strong>Câu mới</span>
            <span><strong>{approvalSummary.alreadyApproved}</strong>Đã có</span>
            <span><strong>{approvalSummary.questionIds.length}</strong>Có thể dùng</span>
          </div>
        )}

        <label htmlFor="examTitle">Tên bài kiểm tra</label>
        <input id="examTitle" value={form.title} onChange={(event) => onFormChange({ ...form, title: event.target.value })} placeholder="VD: Cơ sở dữ liệu - Kiểm tra 15 phút" required />

        <label htmlFor="examDescription">Ghi chú</label>
        <textarea id="examDescription" value={form.description} onChange={(event) => onFormChange({ ...form, description: event.target.value })} rows={3} placeholder="Nội dung ngắn cho giáo viên quản lý nội bộ." />

        <div className="compose-grid">
          <label className="wide-field">
            Bộ đề cương
            <select
              value={form.questionSourceId}
              disabled={sourceLocked}
              onChange={(event) => {
                const nextSource = questionBank.find((bank) => String(bank.id) === event.target.value);
                onFormChange({
                  ...form,
                  questionSourceId: event.target.value,
                  questionCount: nextSource ? String(Math.min(40, nextSource.questionCount)) : "",
                });
              }}
              required
            >
              <option value="">Chọn bộ đề cương</option>
              {questionBank.map((bank) => (
                <option value={bank.id} key={bank.id}>
                  {bank.title} - {bank.questionCount} câu
                </option>
              ))}
            </select>
            {sourceLocked && <span className="form-note">Nguồn đề cương đã khóa vì bài đã có lượt làm.</span>}
          </label>
          <label>
            Số câu lấy
            <input
              type="number"
              min="1"
              max={maxQuestions || undefined}
              value={form.questionCount}
              onChange={(event) => onFormChange({ ...form, questionCount: event.target.value })}
              required
            />
          </label>
          <label>
            Loại bài
            <select value={form.examMode} onChange={(event) => onFormChange({ ...form, examMode: event.target.value as ExamForm["examMode"] })}>
              <option value="practice">Thi thử</option>
              <option value="official">Thi chính thức</option>
              <option value="attendance">Điểm danh</option>
            </select>
          </label>
          <label>
            Lớp áp dụng
            <select value={form.classId} onChange={(event) => onFormChange({ ...form, classId: event.target.value })} required>
              <option value="">Chọn lớp</option>
              {classes.map((classItem) => (
                <option value={classItem.id} key={classItem.id}>{classItem.classCode} - {classItem.className}</option>
              ))}
            </select>
          </label>
          <label>
            Thời gian mở bài
            <input type="datetime-local" value={form.startTime} onChange={(event) => onFormChange({ ...form, startTime: event.target.value })} />
          </label>
          <label>
            Thời lượng phút
            <input type="number" min="5" value={form.durationMinutes} onChange={(event) => onFormChange({ ...form, durationMinutes: event.target.value })} required />
          </label>
          <label>
            Số lần làm
            <select value={form.maxAttemptsPerStudent} onChange={(event) => onFormChange({ ...form, maxAttemptsPerStudent: event.target.value })} required>
              <option value="0">Không giới hạn</option>
              <option value="1">1 lần</option>
              <option value="2">2 lần</option>
              <option value="3">3 lần</option>
            </select>
          </label>
        </div>

        <div className="compose-options">
          <label><input type="checkbox" checked={form.shuffleQuestions} onChange={(event) => onFormChange({ ...form, shuffleQuestions: event.target.checked })} /> Xáo trộn câu khi phát bài</label>
          <label><input type="checkbox" checked={form.shuffleOptions} onChange={(event) => onFormChange({ ...form, shuffleOptions: event.target.checked })} /> Đảo đáp án khi sinh viên làm</label>
          <label><input type="checkbox" checked={form.showResultImmediately} onChange={(event) => onFormChange({ ...form, showResultImmediately: event.target.checked })} /> Hiện điểm sau khi nộp</label>
          <label><input type="checkbox" checked={form.allowReview} onChange={(event) => onFormChange({ ...form, allowReview: event.target.checked })} /> Cho xem lại bài</label>
        </div>

        <button className="primary-btn" type="submit" disabled={isCreating || !selectedSource || requestedCount <= 0}>
          {isCreating ? "Đang lưu..." : submitLabel}
        </button>
        <p className="compact-result">{result}</p>
      </form>

      <aside className="question-bank-picker">
        <div className="compose-head">
          <div>
            <p className="eyebrow">Ngân hàng đề cương</p>
            <h2>Chọn nguồn câu hỏi</h2>
          </div>
          <span className="selected-count">{selectedSource ? `${requestedCount}/${selectedSource.questionCount}` : `0/${questionBank.length}`}</span>
        </div>
        {isLoadingQuestions ? (
          <p className="parser-empty">Đang tải ngân hàng đề cương...</p>
        ) : questionBank.length === 0 ? (
          <p className="parser-empty">Chưa có bộ đề cương nào có câu active. Hãy import đề cương trước.</p>
        ) : (
          <div className="question-bank-list">
            {questionBank.map((bank) => (
              <article className={`bank-question ${String(bank.id) === form.questionSourceId ? "selected" : ""}`} key={bank.id}>
                <button
                  className="bank-question-select"
                  type="button"
                  disabled={sourceLocked}
                  onClick={() => onFormChange({ ...form, questionSourceId: String(bank.id), questionCount: String(Math.min(40, bank.questionCount)) })}
                >
                  <strong>{bank.title}</strong>
                  <span>{bank.questionCount} câu active</span>
                  <small>{bank.sourceName || `Batch #${bank.id}`} - {bank.createdAt}</small>
                </button>
                <button
                  className="bank-delete-btn"
                  type="button"
                  disabled={sourceLocked || deletingBankID === bank.id}
                  onClick={() => onDeleteSource(bank)}
                >
                  {deletingBankID === bank.id ? "Đang xóa" : "Xóa"}
                </button>
              </article>
            ))}
          </div>
        )}
        {selectedSource && (
          <div className="selected-preview">
            <strong>Cách lấy câu</strong>
            <span>{form.shuffleQuestions ? "Lấy ngẫu nhiên khi tạo bài và đảo thứ tự theo từng lượt làm." : "Lấy theo thứ tự trong đề cương."}</span>
          </div>
        )}
      </aside>
    </section>
  );
}

function DuplicateImportModal({
  candidates,
  isApproving,
  onMerge,
  onCreateNew,
}: {
  candidates: ImportDuplicateCandidate[];
  isApproving: boolean;
  onMerge: (candidate: ImportDuplicateCandidate) => void;
  onCreateNew: () => void;
}) {
  const primary = candidates[0];
  return (
    <div className="approval-modal-backdrop" role="dialog" aria-modal="true" aria-label="Kiểm tra đề cương trùng">
      <section className="approval-modal duplicate-import-modal">
        <div>
          <p className="eyebrow">Đề cương đã có</p>
          <h2>File vừa import trùng với ngân hàng hiện tại</h2>
          <p>
            Hệ thống tìm thấy {primary.matchingQuestionCount} câu đã có trong "{primary.title}".
            Nếu đây là bản cập nhật của đề cương cũ, chỉ nên thêm câu mới để tránh phình database.
          </p>
        </div>
        <div className="duplicate-candidate-list">
          {candidates.map((candidate) => (
            <button className="duplicate-candidate" type="button" key={candidate.batchId} disabled={isApproving} onClick={() => onMerge(candidate)}>
              <strong>{candidate.title}</strong>
              <span>{candidate.matchingQuestionCount}/{candidate.existingQuestionCount} câu đã có - ước tính {candidate.newQuestionCount} câu mới</span>
              <small>{candidate.sourceName || `Batch #${candidate.batchId}`} - {candidate.createdAt}</small>
            </button>
          ))}
        </div>
        <div className="approval-modal-actions">
          <button className="primary-btn" type="button" disabled={isApproving} onClick={() => onMerge(primary)}>
            {isApproving ? "Đang lưu..." : "Thêm câu mới vào đề cương đã có"}
          </button>
          <button className="ghost-btn" type="button" disabled={isApproving} onClick={onCreateNew}>Tạo đề cương mới</button>
        </div>
      </section>
    </div>
  );
}

function formSource(form: ExamForm) {
  return Number(form.questionSourceId) > 0;
}

function mergeImportItemIDs(next: ParseResult, previous: ParseResult): ParseResult {
  const previousByOrder = new Map(previous.questions.map((question) => [question.sourceOrder, question.importItemId]));
  return resultFromQuestions(next.questions.map((question, index) => ({
    ...question,
    importItemId: question.importItemId || previousByOrder.get(question.sourceOrder) || previous.questions[index]?.importItemId,
  })), previous.importBatchId);
}

function resultFromQuestions(questions: ParsedQuestion[], importBatchId?: number): ParseResult {
  const total = questions.length;
  const passed = questions.filter((question) => question.status === "pass").length;
  const review = questions.filter((question) => question.status === "review").length;
  const failed = questions.filter((question) => question.status === "fail").length;
  const averageConfidence = total
    ? Math.round(questions.reduce((sum, question) => sum + question.confidence, 0) / total)
    : 0;
  return { importBatchId, questions, summary: { total, passed, review, failed, averageConfidence } };
}
