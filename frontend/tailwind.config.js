/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ide: {
          bg: '#18181b', // zinc-900
          surface: '#27272a', // zinc-800
          border: '#3f3f46', // zinc-700
          hover: '#3f3f46',
          sidebar: '#1e1e24',
          editor: '#141416',
          accent: '#6366f1', // indigo-500
          critic: '#ef4444', // red-500
          security: '#10b981', // emerald-500
          architect: '#3b82f6', // blue-500
          coder: '#a855f7', // purple-500
          arbiter: '#f59e0b', // amber-500
        }
      }
    },
  },
  plugins: [],
}
