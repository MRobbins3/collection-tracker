import type { Config } from "tailwindcss";

export default <Partial<Config>>{
  // Class-based dark mode. The `dark` class is toggled on <html> by a pre-hydration
  // inline script (no flash) and by the useTheme composable at runtime.
  darkMode: "class",
  content: [
    "./components/**/*.{vue,js,ts}",
    "./layouts/**/*.vue",
    "./pages/**/*.vue",
    "./composables/**/*.{js,ts}",
    "./plugins/**/*.{js,ts}",
    "./app.vue",
    "./error.vue",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
};
