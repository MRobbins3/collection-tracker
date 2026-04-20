// Minimal flat-config ESLint setup for Nuxt 3 + Vue 3 + TypeScript.
// The Nuxt module (`@nuxt/eslint`) extends a baseline that matches Nuxt's
// own recommended rules. Local overrides are deliberately small — we lean
// on TypeScript's strict mode for correctness and keep stylistic rules
// minimal to avoid bikeshedding.
import withNuxt from "./.nuxt/eslint.config.mjs";

export default withNuxt({
  rules: {
    "vue/multi-word-component-names": "off", // pages like `index.vue` are fine
    "vue/no-v-html": "warn",
    // Self-closing <input/> is perfectly valid HTML5 and we already ship
    // plenty of them; reasonable stylistic choice, not a correctness issue.
    "vue/html-self-closing": "off",
    "@typescript-eslint/no-unused-vars": [
      "warn",
      { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
    ],
  },
});
