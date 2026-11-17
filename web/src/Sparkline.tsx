type Props = {
  points: { t: string; v: number }[];
  color?: string;
};

export default function Sparkline({ points, color = "#3dd68c" }: Props) {
  const w = 160;
  const h = 42;
  if (!points?.length) {
    return (
      <svg className="spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
        <path d={`M0 ${h / 2} L${w} ${h / 2}`} stroke={color} strokeOpacity="0.25" fill="none" />
      </svg>
    );
  }
  const vals = points.map((p) => p.v);
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const span = max - min || 1;
  const d = points
    .map((p, i) => {
      const x = (i / Math.max(points.length - 1, 1)) * w;
      const y = h - ((p.v - min) / span) * (h - 4) - 2;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg className="spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <path d={d} stroke={color} strokeWidth="2" fill="none" strokeLinecap="round" />
    </svg>
  );
}
