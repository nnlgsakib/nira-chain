/** @type {import('tailwindcss').Config} */
const token = (name) => `rgb(var(--${name}) / <alpha-value>)`;

export default {
  future: {
    hoverOnlyWhenSupported: true,
  },
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        canvas: token("canvas"),
        surface: token("surface"),
        raised: token("raised"),
        line: token("line"),
        ink: token("ink"),
        mute: token("mute"),
        faint: token("faint"),
        accent: {
          DEFAULT: token("accent"),
          soft: token("accent-soft"),
          alt: token("accent-alt"),
          deep: token("accent-deep"),
          ink: token("on-accent"),
        },
        sky: token("sky"),
        up: token("up"),
        down: token("down"),
        warn: token("warn"),
      },
      fontFamily: {
        sans: [
          "Manrope Variable",
          "system-ui",
          "-apple-system",
          "Segoe UI",
          "Roboto",
          "sans-serif",
        ],
        display: ["Unbounded Variable", "Manrope Variable", "system-ui", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "Consolas", "monospace"],
      },
      boxShadow: {
        sheet: "0 -20px 60px rgb(0 0 0 / var(--shadow-alpha))",
        float: "0 18px 40px -12px rgb(0 0 0 / var(--shadow-alpha))",
      },
      transitionTimingFunction: {
        // A strong ease-out for anything entering or responding to a tap,
        // and the drawer curve for sheets and banners.
        out: "cubic-bezier(0.23, 1, 0.32, 1)",
        ios: "cubic-bezier(0.32, 0.72, 0, 1)",
      },
    },
  },
  plugins: [],
};
