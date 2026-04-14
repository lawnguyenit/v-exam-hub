import { Link } from "react-router-dom";

export function Brand({ to = "/" }: { to?: string }) {
  return (
    <Link className="brand" to={to} aria-label="ExamHub">
      <span className="brand-mark">E</span>
      <span>ExamHub</span>
    </Link>
  );
}
