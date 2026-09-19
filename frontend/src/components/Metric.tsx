import type { ReactNode } from "react";

type MetricProps = {
  label: string;
  value: string;
  icon: ReactNode;
  accent: "mint" | "amber" | "aqua" | "coral";
};

const accents = {
  mint: "text-mint bg-mint/12 border-mint/20",
  amber: "text-amber bg-amber/12 border-amber/20",
  aqua: "text-aqua bg-aqua/12 border-aqua/20",
  coral: "text-coral bg-coral/12 border-coral/20"
};

export function Metric({ label, value, icon, accent }: MetricProps) {
  return (
    <div className="surface rounded-lg p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-paper/58">{label}</p>
        <div className={`grid size-10 place-items-center rounded-lg border ${accents[accent]}`}>
          {icon}
        </div>
      </div>
      <p className="mt-4 break-words text-2xl font-black text-paper sm:text-3xl">{value}</p>
    </div>
  );
}
