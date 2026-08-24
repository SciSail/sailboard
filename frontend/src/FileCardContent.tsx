import { useEffect, useState } from "react";
import { GetFileThumbnail } from "../wailsjs/go/main/App";

// Preview-only, same as ImageCardContent (and reusing its .image-preview/.image-placeholder
// styling) — the filename(s) now live in the card footer instead (see App.tsx's metaText), so
// this component has nothing else to render.
export default function FileCardContent({ id }: { id: string }) {
  const [thumb, setThumb] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setThumb(null);
    setFailed(false);
    GetFileThumbnail(id).then(url => { if (!cancelled) setThumb(url); }).catch(() => { if (!cancelled) setFailed(true); });
    return () => { cancelled = true; };
  }, [id]);

  if (failed) return <div className="image-placeholder">缩略图加载失败</div>;
  if (!thumb) return <div className="image-placeholder">加载中…</div>;
  return <img className="image-preview" src={thumb} alt="" />;
}
