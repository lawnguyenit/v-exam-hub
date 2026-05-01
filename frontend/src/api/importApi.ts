import type { ImportParsedQuestion, ImportParseResult } from "./types";

export async function parseTeacherImport(file: File, account?: string) {
  const formData = new FormData();
  formData.append("file", file);
  if (account) {
    formData.append("account", account);
  }

  const response = await fetch("/api/teacher/import/parse", {
    method: "POST",
    body: formData,
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<ImportParseResult>;
}

export async function saveTeacherImportItem(importBatchId: number, question: ImportParsedQuestion) {
  const response = await fetch("/api/teacher/import/items/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, question }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
}

export async function createTeacherImportItem(importBatchId: number, question: ImportParsedQuestion) {
  const response = await fetch("/api/teacher/import/items/create", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, question }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<ImportParsedQuestion>;
}

export async function deleteTeacherImportItem(importBatchId: number, importItemId: number) {
  const response = await fetch("/api/teacher/import/items/delete", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, importItemId }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
}

export async function approveTeacherImportPassItems(importBatchId: number, targetBatchId?: number) {
  const response = await fetch("/api/teacher/import/approve-pass", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, targetBatchId }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{
    importBatchId: number;
    targetBatchId?: number;
    approved: number;
    alreadyApproved: number;
    skipped: number;
    rejected: number;
    questionCount: number;
    questionIds: number[];
  }>;
}

