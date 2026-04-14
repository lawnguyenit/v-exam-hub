import type { ReactNode } from "react";

export function TeacherDetailModal({ content, onClose }: { content: ReactNode; onClose: () => void }) {
  return (
    <div className="modal-backdrop open" onClick={onClose}>
      <section className="student-detail-modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
        <button className="modal-close" type="button" aria-label="Đóng" onClick={onClose}>×</button>
        {content}
      </section>
    </div>
  );
}
