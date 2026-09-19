import type { ButtonHTMLAttributes, ReactNode } from "react";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  tone?: "primary" | "soft" | "danger" | "ghost";
  icon?: ReactNode;
};

const tones = {
  primary:
    "bg-mint text-ink shadow-glow hover:bg-[#8ef0d3] disabled:bg-mint/40",
  soft:
    "bg-paper/10 text-paper hover:bg-paper/16 border border-paper/10 disabled:text-paper/40",
  danger:
    "bg-coral/18 text-[#ffd7d1] hover:bg-coral/28 border border-coral/30",
  ghost: "bg-transparent text-paper/70 hover:bg-paper/8"
};

export function Button({ tone = "primary", icon, children, className = "", ...props }: ButtonProps) {
  return (
    <button
      className={`focus-ring inline-flex min-h-11 items-center justify-center gap-2 rounded-lg px-4 py-2.5 text-sm font-bold transition active:scale-[0.98] disabled:cursor-not-allowed ${tones[tone]} ${className}`}
      {...props}
    >
      {icon}
      <span className="truncate">{children}</span>
    </button>
  );
}
