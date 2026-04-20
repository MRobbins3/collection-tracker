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
        // OS/browser chrome picks the right color per color-scheme.
        { name: "theme-color", content: "#f8fafc", media: "(prefers-color-scheme: light)" },
        { name: "theme-color", content: "#020617", media: "(prefers-color-scheme: dark)" },
      ],
      // Pre-hydration no-FOUC script: resolves theme preference + applies the
      // `dark` class on <html> BEFORE the first paint. Handlers use localStorage
      // + prefers-color-scheme. Wrapped in try/catch so a broken storage policy
      // never blocks the app from loading.
      script: [
        {
          innerHTML: `(function(){try{var s=localStorage.getItem('ct-theme');var p=(s==='light'||s==='dark'||s==='system')?s:'system';var d=p==='dark'||(p==='system'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);}catch(e){}})();`,
          tagPriority: "critical",
          type: "text/javascript",
        },
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
