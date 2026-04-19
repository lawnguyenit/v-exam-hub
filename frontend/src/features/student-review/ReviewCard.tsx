import type { ReviewQuestion } from "../../api";
import { RichQuestionText } from "../../shared/RichQuestionText";

export function ReviewCard({ question, index }: { question: ReviewQuestion; index: number }) {
  return (
    <article className="review-question">
      <p className="eyebrow">Câu {index + 1}</p>
      <h2><RichQuestionText text={question.title} assetBatchId={question.assetBatchId} /></h2>
      <div className="review-answers">
        {question.answers.map((answer, answerIndex) => {
          const correct = answerIndex === question.correctAnswer;
          const selected = answerIndex === question.selectedAnswer;
          return (
            <div className={`review-answer ${correct ? "correct" : selected ? "wrong" : ""}`} key={answer}>
              <span>{String.fromCharCode(65 + answerIndex)}. <RichQuestionText text={answer} assetBatchId={question.assetBatchId} /></span>
              <strong>{correct ? "Đáp án đúng" : selected ? "Bạn đã chọn" : ""}</strong>
            </div>
          );
        })}
      </div>
    </article>
  );
}
