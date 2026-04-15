# Exam Import Pipeline

This document defines how teacher-uploaded exam files should move from messy source material to system-ready questions.

Last checked for provider pricing guidance: 2026-04-14.

---

## 1. Supported input types

Start with these formats:

- `.docx`: best first-class format for teacher-authored exams.
- `.pdf`: common final export format. Handle text PDFs and scanned PDFs differently.
- `.txt`: useful for pasted or manually cleaned question lists.
- `.xlsx` / `.csv`: useful when questions are already row-based.
- `.jpg` / `.jpeg` / `.png`: useful for scanned pages or image-only questions, but these should be treated as OCR/vision jobs.

Support later if needed:

- `.doc`: old Word format. Prefer converting to `.docx` server-side with LibreOffice before parsing.
- `.rtf`: simple text-rich documents.
- `.pptx`: only if teachers really store questions in slides.

Do not accept `.zip` in the first implementation. It complicates virus scanning, nested files, and UX without solving the core exam-import flow.

---

## 2. Default local pipeline

1. Upload file.
2. Store original file metadata in `import_batches`.
3. Extract raw text locally when possible:
   - DOCX: parse paragraphs/tables.
   - PDF text: extract text by page.
   - Scanned PDF/image: OCR.
   - XLSX/CSV: parse rows with columns.
4. Run rule-based parsing:
   - Question starts: `Câu 1:`, `Câu 1`, `1.`, `1)`.
   - Options: `A.`, `A)`, `A:`, `A nội dung`.
   - Answers: `Đáp án: A`, `Đáp án A`, or compact answer lists such as `1.A 2.B`.
5. Score each parsed question.
6. Show preview to the teacher.
7. Teacher approves or edits each item.
8. Approved rows become `question_bank`, `question_bank_options`, `question_bank_tags`, and `question_attachments`.
9. Exam configuration uses `exam_questions` and `exam_versions`.

AI is optional fallback only. The system should work for clean and medium-dirty files without AI.

---

## 3. Parser confidence rule

Each question should be graded before it enters the question bank:

- `pass`: 80-100 points. Can be saved after teacher review.
- `review`: 60-79 points. Parsed but should be inspected.
- `fail`: under 60 points. Must be fixed before saving.

Score inputs:

- recognizable question content
- enough options
- no duplicate option labels
- useful option text
- correct answer found and mapped to an existing option

This is the first protection layer against bad OCR or inconsistent teacher formatting.

---

## 4. Optional AI fallback

Use a two-stage model strategy:

- Default pass: Gemini 2.5 Flash or Gemini 3.1 Flash-Lite for cheaper high-volume extraction.
- Escalation pass: Gemini 2.5 Pro, Gemini 3.1 Pro Preview, GPT-5.4 mini, or GPT-5.4 when the file is scanned, badly formatted, or fails validation.
- Verification pass: run a smaller/cheaper model or deterministic validator to check JSON shape, duplicate options, missing answers, and impossible scores.

Why not use the top model for everything:

- Most teacher files are not reasoning-heavy. They are extraction and normalization work.
- Large models are useful for the worst files, but waste budget on clean DOCX/TXT/XLSX.
- A staged flow gives better cost control and still allows manual teacher approval.

If choosing only one provider for the first implementation, use Gemini for document-heavy imports because its API docs expose native document/multimodal workflows and long-context models. Keep OpenAI as a fallback or validator because structured output handling is strong and easy to enforce.

---

## 5. Output contract

The formatter should return this shape:

```json
{
  "questions": [
    {
      "sourceOrder": 1,
      "type": "single_choice",
      "content": "Question text",
      "options": [
        { "order": 1, "content": "A", "isCorrect": false },
        { "order": 2, "content": "B", "isCorrect": true }
      ],
      "explanation": "Short explanation if present",
      "points": 1,
      "difficulty": 2,
      "subjectCode": "CS",
      "topicCode": "GO",
      "tags": ["go", "goroutine"],
      "confidence": 0.92,
      "warnings": []
    }
  ]
}
```

Rules:

- If the correct answer is missing, set `confidence` low and add a warning.
- If multiple answers look correct but the type says single choice, add a warning.
- Do not invent explanations unless the teacher enables AI-generated explanation.
- Keep original text in `import_items.raw_question_text`.
- Keep normalized output in `import_items.normalized_question_json`.

---

## 6. Cost and daily throughput estimate

Paid ChatGPT/Gemini app subscriptions are not the same as API capacity. Backend usage should use API billing and provider rate limits.

OpenAI API limits are measured across requests and tokens and are tied to organization/project usage tiers. Google says Gemini API limits depend on model, project tier, and active limits visible in AI Studio.

Use this formula:

```text
cost_per_exam =
  input_tokens / 1_000_000 * input_price
  + output_tokens / 1_000_000 * output_price

exams_per_day_by_budget =
  daily_ai_budget / cost_per_exam
```

Baseline assumptions:

- Normal DOCX/TXT/XLSX exam: 20k input tokens + 8k output tokens.
- Dirty/scanned PDF exam: 80k input tokens + 15k output tokens.

Rough standard API cost per exam from listed pricing:

| Model | Normal exam | Dirty/scanned exam |
| --- | ---: | ---: |
| Gemini 2.5 Flash | about $0.03 | about $0.06 |
| Gemini 2.5 Pro | about $0.11 | about $0.25 |
| Gemini 3.1 Pro Preview | about $0.14 | about $0.34 |
| GPT-5.4 mini | about $0.05 | about $0.13 |
| GPT-5.4 | about $0.17 | about $0.43 |

Example daily budgets:

| Daily budget | Gemini 2.5 Flash normal | Gemini 2.5 Pro normal | GPT-5.4 mini normal | GPT-5.4 normal |
| --- | ---: | ---: | ---: | ---: |
| $3/day | about 115 exams | about 28 exams | about 58 exams | about 17 exams |
| $10/day | about 385 exams | about 95 exams | about 196 exams | about 58 exams |

The real cap can be lower if the provider rate-limits RPM/TPM/RPD before budget is reached. Store token usage per import so the app can show live daily usage.

---

## 7. Extra database note

The current schema uses `import_batches.parser_model`, `import_items.ai_confidence`, and `ai_model_runs` for cost reporting:

```text
ai_model_runs(import_batch_id, provider, model, purpose, input_tokens, output_tokens, request_count, estimated_cost_usd, run_status)
```

This keeps provider cost reporting separate from the question bank itself.

---

## 8. Sources checked

- OpenAI API pricing: https://openai.com/api/pricing/
- OpenAI API rate limits: https://developers.openai.com/api/docs/guides/rate-limits
- OpenAI ChatGPT/API billing separation: https://help.openai.com/en/articles/9039756-billing-settings-in-chatgpt-vs-platform
- Gemini API pricing: https://ai.google.dev/gemini-api/docs/pricing
- Gemini API rate limits: https://ai.google.dev/gemini-api/docs/rate-limits
