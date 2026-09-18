import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        "sc-bg":     "#0d0f14",
        "sc-surface":"#131720",
        "sc-border": "rgba(255,255,255,0.07)",
        "sc-text":   "#f0ede8",
        "sc-muted":  "rgba(240,237,232,0.45)",
        "sc-green":  "#4ade80",
        "sc-blue":   "#60a5fa",
        "sc-purple": "#a78bfa",
        "sc-amber":  "#fbbf24",
        "sc-red":    "#f87171",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "-apple-system", "BlinkMacSystemFont", "sans-serif"],
      },
      borderColor: {
        DEFAULT: "rgba(255,255,255,0.07)",
      },
    },
  },
  plugins: [],
} satisfies Config;
