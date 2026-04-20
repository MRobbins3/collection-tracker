// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2025-01-01",
  devtools: { enabled: true },
  modules: ["@nuxtjs/tailwindcss"],
  css: ["~/assets/css/main.css"],
  app: {
    head: {
      title: "Collection Tracker",
      meta: [
        { charset: "utf-8" },
        {
          name: "viewport",
          content: "width=device-width, initial-scale=1, viewport-fit=cover",
        },
        {
          name: "description",
          content: "Track collections of anything — Lego, Funko, coins, stamps, plants, and more.",
        },
        { name: "theme-color", content: "#0f172a" },
      ],
    },
  },
  runtimeConfig: {
    // Server-only. Used by Nitro during SSR to reach the API over its internal
    // docker hostname. Override via NUXT_API_BASE.
    apiBase: process.env.NUXT_API_BASE || "http://api:8080",
    public: {
      // Browser-visible. Override via NUXT_PUBLIC_API_BASE.
      apiBase: process.env.NUXT_PUBLIC_API_BASE || "http://localhost:8080",
    },
  },
  typescript: {
    strict: true,
  },
});
