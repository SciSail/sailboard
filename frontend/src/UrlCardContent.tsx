import { useEffect, useState } from "react";
import { PreviewURL } from "../wailsjs/go/main/App";

type Preview = { title: string; faviconUrl: string; description: string; imageUrl: string };

function safeDomain(value: string) { try { return new URL(value).hostname; } catch { return value; } }

// Preview-only body, same spirit as ImageCardContent/FileCardContent — the raw URL now lives in
// the card footer instead (see App.tsx's metaText). Shows immediately as a placeholder (design
// doc §17's "never block on network") and fills in once the fetch resolves.
//
// Two layouts depending on what came back: a page with an og:image gets that image as a cover
// (like a chat app's link preview) with the title overlaid at the bottom, since the image is the
// more informative preview when it exists; otherwise falls back to the original large-favicon +
// title (+ description, if present) layout. A failed image load falls back the same way, via
// imageFailed — same pattern as faviconFailed below it.
export default function UrlCardContent({ id, text }: { id: string; text: string }) {
  const [preview, setPreview] = useState<Preview | null>(null);
  const [faviconFailed, setFaviconFailed] = useState(false);
  const [imageFailed, setImageFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setPreview(null);
    setFaviconFailed(false);
    setImageFailed(false);
    PreviewURL(id).then(result => { if (!cancelled && result.title) setPreview(result); }).catch(() => {});
    return () => { cancelled = true; };
  }, [id]);

  if (preview?.imageUrl && !imageFailed) {
    return <div className="url-preview url-preview-image">
      <img className="url-cover" src={preview.imageUrl} alt="" onError={() => setImageFailed(true)} />
      <span className="url-cover-title">{preview.title || safeDomain(text)}</span>
    </div>;
  }

  return <div className="url-preview">
    {preview?.faviconUrl && !faviconFailed
      ? <img className="url-favicon" src={preview.faviconUrl} alt="" onError={() => setFaviconFailed(true)} />
      : <div className="url-favicon placeholder">🔗</div>}
    <span className="url-title">{preview?.title || safeDomain(text)}</span>
    {preview?.description && <span className="url-description">{preview.description}</span>}
  </div>;
}
