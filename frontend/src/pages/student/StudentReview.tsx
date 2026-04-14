import { useQuery } from "@tanstack/react-query";
import { Navigate, useSearchParams } from "react-router-dom";
import { getReview } from "../../api";
import { ReviewCard } from "../../features/student-review/ReviewCard";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

export function StudentReview() {
  const auth = useRequiredAuth("student");
  const [params] = useSearchParams();
  const reviewID = params.get("id") || "go-intro";
  const reviewQuery = useQuery({ queryKey: ["review", reviewID], queryFn: () => getReview(reviewID) });

  if (!auth) return <Navigate to="/" replace />;

  return (
    <PageShell backTo="/student">
      <main className="review-page">
        <section className="review-hero">
          <div>
            <p className="eyebrow">Xem lại bài thi</p>
            <h1>{reviewQuery.data?.title || (reviewQuery.isError ? "Không thể tải bài xem lại" : "Đang tải bài thi")}</h1>
          </div>
          <div className="review-score">
            <span>Điểm</span>
            <strong>{reviewQuery.data?.score || "--"}</strong>
            <small>{reviewQuery.data ? `Thời gian làm bài: ${reviewQuery.data.duration}` : "--"}</small>
          </div>
        </section>
        <section className="review-list" aria-label="Danh sách câu đã làm">
          {reviewQuery.data?.questions.map((question, index) => <ReviewCard question={question} index={index} key={question.title} />)}
        </section>
      </main>
    </PageShell>
  );
}
