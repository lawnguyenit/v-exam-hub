import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate } from "react-router-dom";
import { deleteTeacherQuestionBank, getTeacherQuestionBank, type QuestionBankItem } from "../../api";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

export function TeacherQuestionBank() {
  const auth = useRequiredAuth("teacher");
  const queryClient = useQueryClient();
  const [message, setMessage] = useState("");
  const [deletingID, setDeletingID] = useState<number | undefined>();
  const [pendingDelete, setPendingDelete] = useState<QuestionBankItem | null>(null);
  const questionBankQuery = useQuery({
    queryKey: ["teacher-question-bank", auth?.account],
    queryFn: () => getTeacherQuestionBank(auth?.account),
    enabled: Boolean(auth?.account),
  });

  if (!auth) return <Navigate to="/" replace />;

  async function deleteSource(bank: QuestionBankItem) {
    if (!auth?.account) return;
    const account = auth.account;
    setDeletingID(bank.id);
    setMessage("");
    try {
      const result = await deleteTeacherQuestionBank(bank.id, account);
      await queryClient.invalidateQueries({ queryKey: ["teacher-question-bank"] });
      setMessage(`Đã xóa ${result.deletedQuestions} câu và archive ${result.archivedQuestions} câu.`);
      setPendingDelete(null);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Không xóa được bộ đề cương.");
    } finally {
      setDeletingID(undefined);
    }
  }

  const banks = questionBankQuery.data || [];

  return (
    <PageShell backTo="/teacher">
      <main className="question-bank-manager">
        <section className="create-hero question-bank-hero">
          <div>
            <p className="eyebrow">Ngân hàng đề cương</p>
            <h1>Quản lý bộ đề cương đã import</h1>
            <p className="lead">Xem các bộ câu hỏi active, tạo bài kiểm tra từ bộ đề cương, hoặc xóa nguồn không còn dùng.</p>
          </div>
          <div className="question-bank-hero-actions">
            <Link className="primary-btn" to="/teacher/create">Tạo bài kiểm tra</Link>
          </div>
        </section>

        {message && <p className="compact-result">{message}</p>}

        {questionBankQuery.isLoading ? (
          <p className="parser-empty">Đang tải ngân hàng đề cương...</p>
        ) : banks.length === 0 ? (
          <p className="parser-empty">Chưa có bộ đề cương nào có câu active.</p>
        ) : (
          <section className="question-bank-grid">
            {banks.map((bank) => (
              <article className="question-bank-card" key={bank.id}>
                <div>
                  <strong>{bank.title}</strong>
                  <span>{bank.questionCount} câu active</span>
                  <small>{bank.sourceName || `Batch #${bank.id}`} - {bank.createdAt}</small>
                </div>
                <div className="question-bank-card-actions">
                  <Link className="ghost-btn" to={`/teacher/create?source=${bank.id}`}>Dùng tạo bài</Link>
                  <button className="ghost-btn danger" type="button" disabled={deletingID === bank.id} onClick={() => setPendingDelete(bank)}>
                    {deletingID === bank.id ? "Đang xóa" : "Xóa"}
                  </button>
                </div>
              </article>
            ))}
          </section>
        )}
        {pendingDelete && (
          <div className="student-result-backdrop" role="presentation">
            <section className="teacher-confirm-modal" role="dialog" aria-modal="true" aria-label="Xác nhận xóa bộ đề cương">
              <div>
                <p className="eyebrow">Xóa bộ đề cương</p>
                <h2>{pendingDelete.title}</h2>
              </div>
              <p>Các câu đang được bài thi cũ dùng sẽ được archive để giữ lịch sử. Những câu chưa được dùng sẽ bị xóa khỏi ngân hàng active.</p>
              <div className="modal-actions">
                <button className="ghost-btn" type="button" onClick={() => setPendingDelete(null)} disabled={deletingID === pendingDelete.id}>Hủy</button>
                <button className="primary-btn danger" type="button" onClick={() => deleteSource(pendingDelete)} disabled={deletingID === pendingDelete.id}>
                  {deletingID === pendingDelete.id ? "Đang xóa..." : "Xóa bộ đề cương"}
                </button>
              </div>
            </section>
          </div>
        )}
      </main>
    </PageShell>
  );
}
