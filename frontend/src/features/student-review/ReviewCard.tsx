import type { ReviewQuestion } from "../../api";

export function ReviewCard({ question, index }: { question: ReviewQuestion; index: number }) {
  return (
    <article className="review-question">
      <p className="eyebrow">Câu {index + 1}</p>
      <h2>{question.title}</h2>
      <div className="review-answers">
        {question.answers.map((answer, answerIndex) => {
          const correct = answerIndex === question.correctAnswer;
          const selected = answerIndex === question.selectedAnswer;
          return (
            <div className={`review-answer ${correct ? "correct" : selected ? "wrong" : ""}`} key={answer}>
              <span>{String.fromCharCode(65 + answerIndex)}. {answer}</span>
              <strong>{correct ? "Đáp án đúng" : selected ? "Bạn đã chọn" : ""}</strong>
            </div>
          );
        })}
      </div>
    </article>
  );
}
