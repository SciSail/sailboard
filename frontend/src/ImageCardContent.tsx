import { useEffect, useState } from "react";
import { GetImageDataURL } from "../wailsjs/go/main/App";

// Fetches an image item's PNG lazily (and only once per id) rather than inlining every visible
// card's image bytes into the history list response, since images can be multiple MB each.
export default function ImageCardContent({ id, width, height }: { id: string; width: number; height: number }) {
  const [dataUrl, setDataUrl] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setDataUrl(null);
    setFailed(false);
    GetImageDataURL(id).then(url => { if (!cancelled) setDataUrl(url); }).catch(() => { if (!cancelled) setFailed(true); });
    return () => { cancelled = true; };
  }, [id]);

  if (failed) return <div className="image-placeholder">图片加载失败<br />{width} × {height}</div>;
  if (!dataUrl) return <div className="image-placeholder">加载中…<br />{width} × {height}</div>;
  return <img className="image-preview" src={dataUrl} alt={`${width} × ${height}`} />;
}
