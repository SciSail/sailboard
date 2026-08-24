// Purely presentational, like UrlCardContent's fallback path — the raw copied text (already
// trimmed and validated by internal/clipboard/parser.go's matchColorValue before it was ever
// stored as a ContentColor item) is all that's needed, no backend fetch involved. CSS accepts
// #hex and rgb()/rgba() directly but not a bare "(r, g, b)" tuple — or full-width punctuation —
// so this mirrors matchColorValue's own punctuation tolerance (ASCII/full-width parens and
// commas, parens optional entirely) when normalizing into a CSS-valid color.
function toCssColor(text: string): string | null {
  const trimmed = text.trim();
  if (/^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(trimmed)) return trimmed;
  const m = trimmed.match(/^(?:rgba?)?\s*[(（]?\s*(\d{1,3})\s*[,，]\s*(\d{1,3})\s*[,，]\s*(\d{1,3})\s*(?:[,，]\s*([\d.]+)\s*)?[)）]?\s*$/i);
  if (!m) return null;
  const [, r, g, b, a] = m;
  if (a === undefined) return `rgb(${r}, ${g}, ${b})`;
  const alpha = Number(a) > 1 ? Number(a) / 255 : Number(a);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

export default function ColorCardContent({ text }: { text: string }) {
  const color = toCssColor(text);
  return <div className="color-preview" style={{ background: color ?? "transparent" }}>
    <span className="color-value">{text}</span>
  </div>;
}
