import { useState, type ReactNode } from "react";

const imageTokenPattern = /\[H(?:ình|inh)\s+(\d+)(?:[^\]]*)?\]/gi;

export function RichQuestionText({
  text,
  assetBatchId,
  className,
}: {
  text: string;
  assetBatchId?: number;
  className?: string;
}) {
  const nodes = renderQuestionText(text, assetBatchId);
  return <span className={className ? `rich-question-text ${className}` : "rich-question-text"}>{nodes}</span>;
}

function renderQuestionText(text: string, assetBatchId?: number) {
  const parts: ReactNode[] = [];
  let cursor = 0;

  for (const match of text.matchAll(imageTokenPattern)) {
    const index = match.index ?? 0;
    if (index > cursor) {
      parts.push(text.slice(cursor, index));
    }

    const order = Number(match[1]);
    if (assetBatchId && Number.isFinite(order) && order > 0) {
      parts.push(
        <InlineQuestionImage
          key={`asset-${order}-${index}`}
          order={order}
          token={match[0]}
          url={`/api/teacher/import/assets/${assetBatchId}/${order}`}
        />,
      );
    } else {
      parts.push(match[0]);
    }
    cursor = index + match[0].length;
  }

  if (cursor < text.length) {
    parts.push(text.slice(cursor));
  }
  return parts.length ? parts : text;
}

function InlineQuestionImage({ order, token, url }: { order: number; token: string; url: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return <span className="question-image-token">{token}</span>;
  return (
    <img
      alt={`Hinh ${order}`}
      className="question-inline-image"
      loading="lazy"
      onError={() => setFailed(true)}
      src={url}
    />
  );
}
