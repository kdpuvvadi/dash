type Props = {
  points: { t: string; v: number }[];
  color?: string;
};

type Pt = { x: number; y: number };

/** Catmull-Rom → cubic Bezier for a smooth curve through every point. */
function smoothPath(pts: Pt[]): string {
  if (pts.length === 0) return "";
  if (pts.length === 1) return `M${pts[0].x} ${pts[0].y}`;
  if (pts.length === 2) {
    return `M${pts[0].x} ${pts[0].y} L${pts[1].x} ${pts[1].y}`;
  }

  let d = `M${pts[0].x.toFixed(2)} ${pts[0].y.toFixed(2)}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i === 0 ? i : i - 1];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[i + 2] ?? p2;
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;
    d += ` C${cp1x.toFixed(2)} ${cp1y.toFixed(2)}, ${cp2x.toFixed(2)} ${cp2y.toFixed(2)}, ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`;
  }
  return d;
}

export default function Sparkline({ points, color = "#3dd68c" }: Props) {
  const w = 160;
  const h = 42;
  if (!points?.length) {
    return (
      <svg className="spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
        <path
          d={`M0 ${h / 2} C${w * 0.33} ${h / 2}, ${w * 0.66} ${h / 2}, ${w} ${h / 2}`}
          stroke={color}
          strokeOpacity="0.25"
          fill="none"
          strokeWidth="2"
          strokeLinecap="round"
        />
      </svg>
    );
  }

  const vals = points.map((p) => p.v);
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const span = max - min || 1;
  const n = Math.max(points.length - 1, 1);
  const pts: Pt[] = points.map((p, i) => ({
    x: (i / n) * w,
    y: h - ((p.v - min) / span) * (h - 6) - 3,
  }));

  const line = smoothPath(pts);
  const area = `${line} L${pts[pts.length - 1].x.toFixed(2)} ${h} L${pts[0].x.toFixed(2)} ${h} Z`;

  return (
    <svg className="spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <path d={area} fill={color} fillOpacity="0.12" stroke="none" />
      <path
        d={line}
        stroke={color}
        strokeWidth="2"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
