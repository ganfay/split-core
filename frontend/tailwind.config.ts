import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#101418",
        coal: "#151A1F",
        paper: "#F7F2EA",
        mint: "#6FE7C2",
        coral: "#FF7A6B",
        amber: "#F6C453",
        aqua: "#6FB7FF"
      },
      boxShadow: {
        glow: "0 22px 70px rgba(111, 231, 194, 0.18)",
        coral: "0 18px 48px rgba(255, 122, 107, 0.22)"
      },
      fontFamily: {
        sans: [
          "Inter",
          "ui-sans-serif",
          "system-ui",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "sans-serif"
        ]
      }
    }
  },
  plugins: []
} satisfies Config;
