import { useQuery } from "@tanstack/react-query";
import { Navigate, useSearchParams } from "react-router-dom";
import { getExam } from "../../api";
import { ExamWorkspace } from "../../features/student-exam/ExamWorkspace";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

export function StudentExam() {
  const auth = useRequiredAuth("student");
  const [params] = useSearchParams();
  const examID = params.get("id") || "go-basics-demo";

  if (!auth) return <Navigate to="/" replace />;

  const examQuery = useQuery({ queryKey: ["exam", examID], queryFn: () => getExam(examID) });
  return examQuery.data ? <ExamWorkspace auth={auth} exam={examQuery.data} /> : (
    <PageShell backTo="/student">
      <main className="exam-page">
        <article className="exam-panel">{examQuery.isError ? "Không thể tải bài kiểm tra" : "Đang tải bài kiểm tra..."}</article>
      </main>
    </PageShell>
  );
}
