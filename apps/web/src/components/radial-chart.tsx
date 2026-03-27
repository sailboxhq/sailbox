import { Cell, Pie, PieChart, ResponsiveContainer } from "recharts";

interface RadialChartProps {
  value: number;
  max: number;
  label: string;
  sublabel?: string;
  color?: string;
  size?: number;
}

export function RadialChart({
  value,
  max,
  label,
  sublabel,
  color = "#6d5cdb",
  size = 120,
}: RadialChartProps) {
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  const data = [{ value: pct }, { value: 100 - pct }];

  const pctColor = pct > 90 ? "#ef4444" : pct > 70 ? "#eab308" : color;

  return (
    <div className="flex flex-col items-center gap-1">
      <div className="relative" style={{ width: size, height: size }}>
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              cx="50%"
              cy="50%"
              innerRadius="75%"
              outerRadius="95%"
              startAngle={90}
              endAngle={-270}
              strokeWidth={0}
              isAnimationActive={false}
            >
              <Cell fill={pctColor} />
              <Cell fill="var(--color-muted, #e5e7eb)" fillOpacity={0.3} />
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-lg font-bold tracking-tight">{pct.toFixed(0)}%</span>
        </div>
      </div>
      <p className="text-xs font-medium">{label}</p>
      {sublabel && <p className="text-[10px] text-muted-foreground">{sublabel}</p>}
    </div>
  );
}

interface StatusDonutProps {
  segments: { label: string; value: number; color: string }[];
  total: number;
  label: string;
  centerText: string;
  size?: number;
}

export function StatusDonut({ segments, total, label, centerText, size = 120 }: StatusDonutProps) {
  const data = segments.filter((s) => s.value > 0);
  if (data.length === 0) {
    data.push({ label: "none", value: 1, color: "var(--color-muted, #e5e7eb)" });
  }

  return (
    <div className="flex flex-col items-center gap-1">
      <div className="relative" style={{ width: size, height: size }}>
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              cx="50%"
              cy="50%"
              innerRadius="75%"
              outerRadius="95%"
              startAngle={90}
              endAngle={-270}
              strokeWidth={0}
              isAnimationActive={false}
            >
              {data.map((entry, i) => (
                <Cell key={`cell-${i}`} fill={entry.color} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-lg font-bold tracking-tight">{centerText}</span>
        </div>
      </div>
      <p className="text-xs font-medium">{label}</p>
      <p className="text-[10px] text-muted-foreground">{total} total</p>
    </div>
  );
}
