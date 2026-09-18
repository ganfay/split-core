import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        "sc-bg":     "var(--sc-bg)",
        "sc-surface":"var(--sc-surface)",
        "sc-border": "var(--sc-border)",
        "sc-text":   "var(--sc-text)",
        "sc-muted":  "var(--sc-muted)",
        "sc-green":  "var(--sc-green)",
        "sc-blue":   "var(--sc-blue)",
        "sc-purple": "var(--sc-purple)",
        "sc-amber":  "var(--sc-amber)",
        "sc-red":    "var(--sc-red)",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "-apple-system", "BlinkMacSystemFont", "sans-serif"],
      },
      borderColor: {
        DEFAULT: "var(--sc-border)",
      },
    },
  },
  plugins: [],
} satisfies Config;
