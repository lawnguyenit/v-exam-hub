export type ParsedOption = {
  label: string;
  content: string;
};

export type ParsedQuestion = {
  importItemId?: number;
  sourceOrder: number;
  content: string;
  options: ParsedOption[];
  correctLabel?: string;
  confidence: number;
  status: "pass" | "review" | "fail";
  warnings: string[];
};

export type ParseSummary = {
  total: number;
  passed: number;
  review: number;
  failed: number;
  averageConfidence: number;
};

export type ParseResult = {
  questions: ParsedQuestion[];
  summary: ParseSummary;
};

type DraftQuestion = {
  importItemId?: number;
  sourceOrder: number;
  contentParts: string[];
  options: ParsedOption[];
  correctLabel?: string;
  expectOptions?: boolean;
};

const questionLinePattern = /^\s*(?:câu|cau|question|q)\s*(\d{1,4})\s*[:.)/\]-]*\s*(.*)$/i;
const numberedQuestionPattern = /^\s*(\d{1,4})\s*[/.)]+\s*(.{1,})$/;
const looseNumberedQuestionPattern = /^\s*(\d{1,4})\s+(.{8,})$/;
const unnumberedQuestionPattern = /^\s*(?:câu\s*hỏi|cau\s*hoi|question)\s*[:.)/\]-]*\s*(.*)$/i;
const optionLinePattern = /^\s*([A-H])\s*[.)\]:-]?\s+(.+)$/;
const lowercaseListLinePattern = /^\s*[a-h]\s*[-.)]\s+.+$/;
const imagePlaceholderPattern = /^\s*\[Hình\s+\d+\]\s*$/;
const selectOnePattern = /^\s*(?:select\s+one|chọn\s+một|chon\s+mot)\s*:?$/i;
const answerLinePattern = /^\s*(?:đáp\s*án|dap\s*an|đ\/a|d\/a|answer|key)\s*[:.)\]-]?\s*(.+)$/i;
const redAnswerLinePattern = /^\s*\[đáp án màu đỏ\]\s*(.+)$/i;

export function parseExamText(source: string): ParseResult {
  const lines = source.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
  const drafts: DraftQuestion[] = [];
  const answerMap = new Map<number, string>();
  let current: DraftQuestion | undefined;
  let fallbackOrder = 1;

  function pushCurrent() {
    if (!current) return;
    trimDraft(current);
    if (current.contentParts.length || current.options.length) drafts.push(current);
  }

  for (const rawLine of lines) {
    const line = rawLine.replace(/\s+/g, " ").trim();
    if (!line || shouldSkipParserLine(line)) continue;

    if (selectOnePattern.test(line)) {
      if (current) current.expectOptions = true;
      continue;
    }

    if (imagePlaceholderPattern.test(line)) {
      if (current) appendVisualLine(current, line);
      continue;
    }

    const redAnswer = line.match(redAnswerLinePattern);
    if (redAnswer) {
      if (current) applyStyledAnswer(current, redAnswer[1]);
      continue;
    }

    const answerLine = line.match(answerLinePattern);
    if (answerLine) {
      collectAnswerPairs(answerLine[1], answerMap);
      const singleAnswer = answerLine[1].match(/\b([A-H])\b/i);
      if (current && singleAnswer) current.correctLabel = singleAnswer[1].toUpperCase();
      continue;
    }

    const questionMatch = looksLikeAnswerList(line)
      ? undefined
      : line.match(questionLinePattern) || line.match(numberedQuestionPattern) || line.match(looseNumberedQuestionPattern);
    if (questionMatch) {
      pushCurrent();
      const parsedOrder = Number(questionMatch[1]);
      current = {
        sourceOrder: Number.isFinite(parsedOrder) ? parsedOrder : fallbackOrder,
        contentParts: [questionMatch[2].trim()].filter(Boolean),
        options: [],
      };
      fallbackOrder = current.sourceOrder + 1;
      continue;
    }

    const unnumberedQuestion = line.match(unnumberedQuestionPattern);
    if (unnumberedQuestion?.[1]?.trim()) {
      pushCurrent();
      current = {
        sourceOrder: fallbackOrder,
        contentParts: [unnumberedQuestion[1].trim()],
        options: [],
      };
      fallbackOrder += 1;
      continue;
    }

    if (looksLikeLooseQuestion(line, current)) {
      pushCurrent();
      current = {
        sourceOrder: fallbackOrder,
        contentParts: [line],
        options: [],
      };
      fallbackOrder += 1;
      continue;
    }

    const optionMatch = line.match(optionLinePattern);
    if (current && optionMatch) {
      current.options.push({
        label: optionMatch[1].toUpperCase(),
        content: optionMatch[2].trim(),
      });
      current.expectOptions = true;
      continue;
    }

    if (current && shouldInferUnlabeledOption(current, line)) {
      current.options.push({
        label: optionLabel(current.options.length),
        content: line,
      });
      continue;
    }

    collectAnswerPairs(line, answerMap);

    if (!current) continue;
    if (current.options.length > 0) {
      const lastOption = current.options[current.options.length - 1];
      lastOption.content = `${lastOption.content} ${line}`.trim();
    } else {
      current.contentParts.push(line);
    }
  }

  pushCurrent();

  const questions = applyCrossQuestionWarnings(drafts.map((draft) => scoreQuestion({
    ...draft,
    correctLabel: draft.correctLabel || answerMap.get(draft.sourceOrder),
  })));

  return { questions, summary: summarize(questions) };
}

export function recheckParsedQuestions(questions: ParsedQuestion[]): ParseResult {
  const rescored = applyCrossQuestionWarnings(questions.map((question) => scoreQuestion({
    sourceOrder: question.sourceOrder,
    importItemId: question.importItemId,
    contentParts: [question.content],
    options: question.options,
    correctLabel: question.correctLabel,
  })));
  return { questions: rescored, summary: summarize(rescored) };
}

function trimDraft(draft: DraftQuestion) {
  draft.contentParts = draft.contentParts.map((part) => part.trim()).filter(Boolean);
  draft.options = draft.options
    .map((option) => ({ ...option, content: option.content.trim() }))
    .filter((option) => option.content.length > 0);
}

function shouldSkipParserLine(line: string) {
  if (line.length >= 5 && line.replace(/-/g, "").trim() === "") return true;
  const lower = line.toLowerCase();
  return lower === "đoạn văn câu hỏi" || lower === "doan van cau hoi";
}

function shouldInferUnlabeledOption(current: DraftQuestion, line: string) {
  if (current.options.length >= 8 || lowercaseListLinePattern.test(line)) return false;
  if (current.expectOptions) return true;
  if (current.options.length > 0 && current.options.length < 4 && line.length <= 120) return true;
  return current.contentParts.length === 1 && current.options.length === 0 && line.length <= 120;
}

function looksLikeLooseQuestion(line: string, current?: DraftQuestion) {
  return line.trim().endsWith("?") && (!current || current.options.length >= 4);
}

function appendVisualLine(current: DraftQuestion, line: string) {
  if (current.options.length > 0) {
    const lastOption = current.options[current.options.length - 1];
    lastOption.content = `${lastOption.content} ${line}`.trim();
    return;
  }
  current.contentParts.push(line);
  current.expectOptions = true;
}

function applyStyledAnswer(current: DraftQuestion, answerText: string) {
  const optionMatch = answerText.match(optionLinePattern);
  if (optionMatch) {
    current.correctLabel = optionMatch[1].toUpperCase();
    return;
  }
  const needle = normalizeAnswerText(answerText);
  const option = current.options.find((item) => normalizeAnswerText(item.content) === needle);
  if (option) current.correctLabel = option.label;
}

function normalizeAnswerText(value: string) {
  return value.replace(/\s+/g, " ").trim().toLowerCase();
}

function optionLabel(index: number) {
  return String.fromCharCode("A".charCodeAt(0) + index);
}

function collectAnswerPairs(line: string, answerMap: Map<number, string>) {
  const pairPattern = /(?:câu|cau)?\s*(\d{1,4})\s*[.)/\]:-]?\s*([A-H])\b/gi;
  for (const match of line.normalize("NFC").matchAll(pairPattern)) {
    answerMap.set(Number(match[1]), match[2].toUpperCase());
  }
}

function looksLikeAnswerList(line: string) {
  const lower = line.toLowerCase();
  return lower.includes("đáp án") || lower.includes("dap an") || lower.includes("answer");
}

function scoreQuestion(draft: DraftQuestion): ParsedQuestion {
  const content = draft.contentParts.join(" ").replace(/\s+/g, " ").trim();
  const warnings: string[] = [];
  let confidence = 0;

  if (content.length >= 12) confidence += 25;
  else warnings.push("Nội dung câu hỏi quá ngắn hoặc bị tách sai.");

  if (draft.options.length >= 4) {
    confidence += 30;
    if (draft.options.length > 4) warnings.push("Có hơn 4 lựa chọn, cần giáo viên xác nhận câu này không bị dính thêm dòng.");
  }
  else if (draft.options.length >= 2) {
    confidence += 18;
    warnings.push("Ít hơn 4 lựa chọn, cần kiểm tra.");
  } else {
    warnings.push("Không đủ lựa chọn A/B/C/D.");
  }

  const labels = draft.options.map((option) => option.label);
  const uniqueLabels = new Set(labels);
  if (uniqueLabels.size === labels.length) confidence += 15;
  else warnings.push("Có lựa chọn bị trùng ký tự.");

  const hasUsefulOptions = draft.options.every((option) => option.content.length >= 2);
  if (hasUsefulOptions && draft.options.length > 0) confidence += 10;
  else warnings.push("Có lựa chọn quá ngắn hoặc rỗng.");

  const hasValidAnswer = !!draft.correctLabel && uniqueLabels.has(draft.correctLabel);
  if (hasValidAnswer) confidence += 20;
  else if (draft.correctLabel) warnings.push(`Đáp án ${draft.correctLabel} không khớp lựa chọn đã tách.`);
  else warnings.push("Chưa tìm thấy đáp án đúng.");

  if (draft.options.length > 4 && confidence >= 80) confidence = 79;
  if (!hasValidAnswer && confidence >= 80) confidence = 79;
  const status = confidence >= 80 ? "pass" : confidence >= 60 ? "review" : "fail";

  return {
    sourceOrder: draft.sourceOrder,
    importItemId: draft.importItemId,
    content,
    options: draft.options,
    correctLabel: draft.correctLabel,
    confidence,
    status,
    warnings,
  };
}

function applyCrossQuestionWarnings(questions: ParsedQuestion[]): ParsedQuestion[] {
  const counts = new Map<string, number>();
  for (const question of questions) {
    const key = duplicateQuestionKey(question);
    counts.set(key, (counts.get(key) || 0) + 1);
  }

  return questions.map((question) => {
    if ((counts.get(duplicateQuestionKey(question)) || 0) <= 1) return question;
    const confidence = Math.min(question.confidence, 79);
    const status: ParsedQuestion["status"] = confidence >= 60 ? "review" : "fail";
    return {
      ...question,
      confidence,
      status,
      warnings: [
        ...question.warnings,
        "Trùng số câu với một câu khác. Không tự xoá để tránh mất câu thật; giáo viên cần gộp, đổi số, hoặc xoá bản thừa.",
      ],
    };
  });
}

function duplicateQuestionKey(question: ParsedQuestion) {
  return `${question.sourceOrder}|${normalizeAnswerText(question.content)}`;
}

function summarize(questions: ParsedQuestion[]): ParseSummary {
  const total = questions.length;
  const passed = questions.filter((question) => question.status === "pass").length;
  const review = questions.filter((question) => question.status === "review").length;
  const failed = questions.filter((question) => question.status === "fail").length;
  const averageConfidence = total
    ? Math.round(questions.reduce((sum, question) => sum + question.confidence, 0) / total)
    : 0;

  return { total, passed, review, failed, averageConfidence };
}
