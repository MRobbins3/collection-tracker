import type { Config } from "tailwindcss";

export default <Partial<Config>>{
  // Class-based dark mode. The `dark` class is toggled on <html> by a pre-hydration
  // inline script (no flash) and by the useTheme composable at runtime.
  darkMode: "class",
  content: [
    "./app/components/**/*.{vue,js,ts}",
    "./app/layouts/**/*.vue",
    "./app/pages/**/*.vue",
    "./app/composables/**/*.{js,ts}",
    "./app/plugins/**/*.{js,ts}",
    "./app/app.vue",
    "./app/error.vue",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
};
