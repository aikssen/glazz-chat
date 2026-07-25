import Link from "next/link";
import type { CSSProperties } from "react";

export function GlazzMark({ size = 32 }: { size?: number }) {
  return (
    <span
      className="glazz-mark"
      aria-hidden="true"
      style={{ "--glazz-mark-size": `${size}px` } as CSSProperties}
    >
      G
    </span>
  );
}

export function GlazzWordmark() {
  return (
    <Link href="/" className="wordmark" aria-label="Glazz">
      <GlazzMark />
      <span>Glazz</span>
    </Link>
  );
}
