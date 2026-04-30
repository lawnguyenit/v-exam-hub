import katex from "katex";
import { useState, type ReactNode } from "react";

const richTokenPattern = /(\[H(?:inh|ình|Ã¬nh)\s+(\d+)(?:[^\]]*)?\])|\\\[((?:.|\n)+?)\\\]|\\\(((?:.|\n)+?)\\\)/gi;

export function RichQuestionText({
  text,
  assetBatchId,
  className,
  mathLayout = "auto",
}: {
  text: string;
  assetBatchId?: number;
  className?: string;
  mathLayout?: "auto" | "block";
}) {
  const nodes = renderQuestionText(text, assetBatchId, mathLayout);
  return <span className={className ? `rich-question-text ${className}` : "rich-question-text"}>{nodes}</span>;
}

function renderQuestionText(text: string, assetBatchId?: number, mathLayout: "auto" | "block" = "auto") {
  const parts: ReactNode[] = [];
  let cursor = 0;

  for (const match of text.matchAll(richTokenPattern)) {
    const index = match.index ?? 0;
    if (index > cursor) {
      parts.push(text.slice(cursor, index));
    }

    if (match[1]) {
      const order = Number(match[2]);
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
    } else if (match[3] || match[4]) {
      const math = match[3] || match[4] || "";
      const display = mathLayout === "block" || Boolean(match[3]) || shouldPromoteMathToBlock(math);
      parts.push(<InlineMath key={`math-${index}`} display={display} fallback={match[0]} math={math} />);
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

function shouldPromoteMathToBlock(math: string) {
  const normalized = math.replace(/\s+/g, " ").trim();
  const blockOperators = /\\(?:begin|matrix|pmatrix|bmatrix|vmatrix|cases|array|aligned|align|gathered|split)\b/;
  const structuralBraces = /\\left\s*\\?\{|\{[^{}]*\\(?:frac|dfrac|tfrac|begin|matrix|cases|array)[^{}]*\}/;
  const manyFractions = (normalized.match(/\\(?:frac|dfrac|tfrac)\b/g) ?? []).length >= 2;
  return normalized.length > 90 || blockOperators.test(normalized) || structuralBraces.test(normalized) || manyFractions;
}

function InlineMath({ math, display, fallback }: { math: string; display: boolean; fallback: string }) {
  try {
    const html = katex.renderToString(math, {
      displayMode: display,
      throwOnError: false,
      strict: false,
      trust: false,
    });
    return (
      <span
        className={display ? "question-math question-math-display" : "question-math"}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    );
  } catch {
    return <span className="question-math-fallback">{fallback}</span>;
  }
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
