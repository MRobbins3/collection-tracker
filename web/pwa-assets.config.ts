import { defineConfig, minimal2023Preset } from "@vite-pwa/assets-generator/config";

// Generates the full PWA icon set (192, 512, maskable, apple-touch-icon,
// favicon) from public/logo.svg. Run with `pnpm exec pwa-assets-generator`.
export default defineConfig({
  headLinkOptions: {
    preset: "2023",
  },
  preset: minimal2023Preset,
  images: ["public/logo.svg"],
});
