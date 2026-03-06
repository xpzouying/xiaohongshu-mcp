/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        "brand-ink": "#15222e",
        "brand-primary": "#2563eb",
        "brand-accent": "#f59e0b",
        "brand-soft": "#eef4ff"
      }
    }
  },
  plugins: []
};

