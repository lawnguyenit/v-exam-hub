import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import {
  deleteTeacherClass,
  getTeacherClassDetail,
  getTeacherClasses,
  removeTeacherClassMember,
  updateTeacherClass,
  type TeacherClass,
} from "../../api";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

export function TeacherClasses() {
  const auth = useRequiredAuth("teacher");
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [selectedClassID, setSelectedClassID] = useState(0);
  const [message, setMessage] = useState("");
  const [pendingDeleteClass, setPendingDeleteClass] = useState<TeacherClass | null>(null);
  const [pendingRemoveMember, setPendingRemoveMember] = useState<{ userId: number; fullName: string } | null>(null);
  const [editForm, setEditForm] = useState({ classCode: "", className: "" });

  const classQuery = useQuery({ queryKey: ["teacher-classes"], queryFn: getTeacherClasses, enabled: Boolean(auth) });
  const classes = classQuery.data || [];
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return classes;
    return classes.filter((item) => `${item.classCode} ${item.className}`.toLowerCase().includes(needle));
  }, [classes, query]);
  const activeClassID = selectedClassID || filtered[0]?.id || 0;
  const detailQuery = useQuery({
    queryKey: ["teacher-class-detail", activeClassID],
    queryFn: () => getTeacherClassDetail(activeClassID),
    enabled: Boolean(auth && activeClassID),
  });
  const detail = detailQuery.data;

  useEffect(() => {
    if (!detail) return;
    setEditForm({ classCode: detail.classCode, className: detail.className });
  }, [detail]);

  const updateMutation = useMutation({
    mutationFn: () => updateTeacherClass(activeClassID, editForm),
    onSuccess: async () => {
      setMessage("Đã lưu thông tin lớp.");
      await queryClient.invalidateQueries({ queryKey: ["teacher-classes"] });
      await queryClient.invalidateQueries({ queryKey: ["teacher-class-detail", activeClassID] });
    },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Không sửa được lớp."),
  });

  const deleteMutation = useMutation({
    mutationFn: (classID: number) => deleteTeacherClass(classID),
    onSuccess: async () => {
      setMessage("Đã xóa lớp khỏi danh sách active.");
      setPendingDeleteClass(null);
      setSelectedClassID(0);
      await queryClient.invalidateQueries({ queryKey: ["teacher-classes"] });
    },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Không xóa được lớp."),
  });

  const removeMutation = useMutation({
    mutationFn: (payload: { classID: number; userID: number }) => removeTeacherClassMember(payload.classID, payload.userID),
    onSuccess: async () => {
      setMessage("Đã xóa sinh viên khỏi lớp.");
      setPendingRemoveMember(null);
      await queryClient.invalidateQueries({ queryKey: ["teacher-classes"] });
      await queryClient.invalidateQueries({ queryKey: ["teacher-class-detail", activeClassID] });
    },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Không xóa được sinh viên khỏi lớp."),
  });

  if (!auth) return <Navigate to="/" replace />;

  function saveClass(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("Đang lưu thông tin lớp...");
    updateMutation.mutate();
  }

  return (
    <PageShell backTo="/teacher">
      <main className="class-manager-page">
        <section className="class-manager-hero">
          <div>
            <p className="eyebrow">Quản lý lớp</p>
            <h1>Danh sách lớp</h1>
            <p className="lead">Tìm lớp, chỉnh thông tin, quản lý thành viên và theo dõi nhanh kết quả bài kiểm tra.</p>
          </div>
          <Link className="primary-btn" to="/teacher/students">Import sinh viên</Link>
        </section>

        <div className="class-manager-layout">
          <aside className="class-list-panel" aria-label="Danh sách lớp">
            <div>
              <p className="eyebrow">Lớp đã tạo</p>
              <h2>Danh sách lớp</h2>
            </div>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm mã lớp hoặc tên lớp" />
            <div className="class-list">
              {filtered.map((item) => (
                <button
                  className={`class-list-item ${item.id === activeClassID ? "active" : ""}`}
                  type="button"
                  key={item.id}
                  onClick={() => setSelectedClassID(item.id)}
                >
                  <strong>{item.classCode}</strong>
                  <span>{item.className}</span>
                  <small>{item.memberCount || 0} sinh viên - {item.examCount || 0} bài kiểm tra</small>
                </button>
              ))}
              {!filtered.length && <p className="empty-note">Không có lớp phù hợp bộ lọc.</p>}
            </div>
          </aside>

          <section className="class-workspace">
            {detail ? (
              <>
                <section className="class-summary-panel">
                  <div className="class-title-block">
                    <p className="eyebrow">Thông tin lớp</p>
                    <h2>{detail.classCode}</h2>
                    <span>{detail.className}</span>
                  </div>
                  <div className="class-summary-stats">
                    <span><strong>{detail.memberCount}</strong>Sinh viên</span>
                    <span><strong>{detail.examCount}</strong>Bài kiểm tra</span>
                    <span><strong>{detail.averageScore}</strong>Điểm TB</span>
                  </div>
                </section>

                <form className="class-edit-strip" onSubmit={saveClass}>
                  <label>
                    Mã lớp
                    <input value={editForm.classCode} onChange={(event) => setEditForm({ ...editForm, classCode: event.target.value })} required />
                  </label>
                  <label>
                    Tên lớp
                    <input value={editForm.className} onChange={(event) => setEditForm({ ...editForm, className: event.target.value })} required />
                  </label>
                  <div className="class-edit-actions">
                    <button className="ghost-btn danger" type="button" onClick={() => setPendingDeleteClass(detail)}>Xóa lớp</button>
                    <button className="primary-btn" type="submit" disabled={updateMutation.isPending}>{updateMutation.isPending ? "Đang lưu" : "Lưu"}</button>
                  </div>
                </form>

                <section className="class-data-section">
                  <div className="class-section-head">
                    <div>
                      <p className="eyebrow">Thành viên</p>
                      <h2>Sinh viên trong lớp</h2>
                    </div>
                    <span>{detail.members.length} active</span>
                  </div>
                  <div className="class-member-table">
                    <div className="class-member-row header">
                      <span>Mã SV</span><span>Họ tên</span><span>Tài khoản</span><span>Lượt làm</span><span>Điểm cao nhất</span><span></span>
                    </div>
                    {detail.members.map((member) => (
                      <div className="class-member-row" key={member.userId}>
                        <span>{member.studentCode}</span>
                        <strong>{member.fullName}</strong>
                        <span>{member.username}</span>
                        <span>{member.attemptCount}</span>
                        <span>{member.bestScore}</span>
                        <button className="table-action danger" type="button" onClick={() => setPendingRemoveMember({ userId: member.userId, fullName: member.fullName })}>Xóa</button>
                      </div>
                    ))}
                    {detail.members.length === 0 && <p className="empty-note">Lớp chưa có sinh viên active.</p>}
                  </div>
                </section>

                <section className="class-data-section">
                  <div className="class-section-head">
                    <div>
                      <p className="eyebrow">Bài kiểm tra</p>
                      <h2>Kết quả theo lớp</h2>
                    </div>
                  </div>
                  <div className="class-exam-list">
                    {detail.exams.map((exam) => (
                      <article key={exam.id}>
                        <strong>{exam.title}</strong>
                        <span>{exam.status} - đã nộp {exam.submitted}/{exam.total} - điểm TB {exam.average}</span>
                      </article>
                    ))}
                    {detail.exams.length === 0 && <p className="empty-note">Chưa có bài kiểm tra nào gán cho lớp này.</p>}
                  </div>
                </section>
              </>
            ) : (
              <p className="parser-empty">{classQuery.isLoading ? "Đang tải danh sách lớp..." : "Chưa có lớp nào. Hãy import danh sách sinh viên để tạo lớp."}</p>
            )}

            {message && <p className="student-import-message">{message}</p>}
          </section>
        </div>
      </main>

      {pendingDeleteClass && (
        <div className="modal-backdrop open" role="dialog" aria-modal="true" aria-label="Xác nhận xóa lớp">
          <section className="teacher-confirm-modal">
            <p className="eyebrow">Xóa lớp</p>
            <h2>{pendingDeleteClass.classCode}</h2>
            <p>Lớp sẽ bị ẩn khỏi danh sách active và các thành viên sẽ được đưa về trạng thái inactive. Lịch sử bài thi vẫn được giữ.</p>
            <div className="modal-actions">
              <button className="ghost-btn" type="button" onClick={() => setPendingDeleteClass(null)}>Hủy</button>
              <button className="primary-btn danger" type="button" onClick={() => deleteMutation.mutate(pendingDeleteClass.id)} disabled={deleteMutation.isPending}>
                {deleteMutation.isPending ? "Đang xóa" : "Xóa lớp"}
              </button>
            </div>
          </section>
        </div>
      )}

      {pendingRemoveMember && (
        <div className="modal-backdrop open" role="dialog" aria-modal="true" aria-label="Xác nhận xóa sinh viên khỏi lớp">
          <section className="teacher-confirm-modal">
            <p className="eyebrow">Xóa thành viên</p>
            <h2>{pendingRemoveMember.fullName}</h2>
            <p>Sinh viên sẽ không còn nằm trong lớp active này. Tài khoản sinh viên và lịch sử làm bài không bị xóa.</p>
            <div className="modal-actions">
              <button className="ghost-btn" type="button" onClick={() => setPendingRemoveMember(null)}>Hủy</button>
              <button
                className="primary-btn danger"
                type="button"
                onClick={() => removeMutation.mutate({ classID: activeClassID, userID: pendingRemoveMember.userId })}
                disabled={removeMutation.isPending}
              >
                {removeMutation.isPending ? "Đang xóa" : "Xóa khỏi lớp"}
              </button>
            </div>
          </section>
        </div>
      )}
    </PageShell>
  );
}
