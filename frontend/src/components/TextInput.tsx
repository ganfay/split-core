import type { InputHTMLAttributes } from "react";

type TextInputProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string;
};

export function TextInput({ label, className = "", ...props }: TextInputProps) {
  return (
    <label className="grid gap-2 text-sm text-paper/72">
      <span className="font-semibold">{label}</span>
      <input
        className={`focus-ring min-h-12 w-full rounded-lg border border-paper/10 bg-ink/55 px-4 text-base text-paper placeholder:text-paper/34 ${className}`}
        {...props}
      />
    </label>
  );
}
