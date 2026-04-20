// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2025-01-01",
  devtools: { enabled: true },
  modules: ["@nuxtjs/tailwindcss", "@nuxt/icon", "@vite-pwa/nuxt"],
  css: ["~/assets/css/main.css"],
  icon: {
    // Prefer server-rendered svg, which ships HTML the browser can paint
    // immediately (no icon-name → svg round-trip on hydration).
    mode: "svg",
  },
  pwa: {
    strategies: "generateSW",
    registerType: "autoUpdate",
    manifest: {
      name: "Collection Tracker",
      short_name: "Collection",
      description: "Track collections of anything — Lego, Funko, coins, stamps, plants, and more.",
      theme_color: "#0f172a",
      background_color: "#f8fafc",
      display: "standalone",
      orientation: "portrait",
      scope: "/",
      start_url: "/",
      icons: [
        { src: "pwa-64x64.png", sizes: "64x64", type: "image/png" },
        { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
        { src: "pwa-512x512.png", sizes: "512x512", type: "image/png" },
        {
          src: "maskable-icon-512x512.png",
          sizes: "512x512",
          type: "image/png",
          purpose: "maskable",
        },
      ],
    },
    workbox: {
      globPatterns: ["**/*.{js,css,html,ico,png,svg,woff2}"],
      navigateFallback: "/",
      runtimeCaching: [
        {
          // Public categories are the only API surface a non-signed-in user
          // browses, so cache-first lets the category list open offline.
          urlPattern: /\/categories(\?.*)?$/,
          handler: "StaleWhileRevalidate",
          options: {
            cacheName: "api-categories",
            expiration: { maxEntries: 8, maxAgeSeconds: 60 * 60 * 24 },
          },
        },
        {
          urlPattern: /\/categories\/[a-z0-9-]+$/,
          handler: "StaleWhileRevalidate",
          options: {
            cacheName: "api-category-detail",
            expiration: { maxEntries: 30, maxAgeSeconds: 60 * 60 * 24 },
          },
        },
      ],
    },
    devOptions: {
      // Registering the SW in dev makes the install story testable without a
      // production build. Safe because navigateFallback points at '/'.
      enabled: true,
      type: "module",
      suppressWarnings: true,
    },
  },
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
        // Standalone-mode hints. The non-prefixed `mobile-web-app-capable` is
        // the current standard (Chrome/Android); the apple-prefixed version
        // stays for older iOS Safari releases that haven't picked up the
        // unprefixed name yet. Chrome warns if the apple one is shipped alone.
        { name: "mobile-web-app-capable", content: "yes" },
        { name: "apple-mobile-web-app-capable", content: "yes" },
        { name: "apple-mobile-web-app-status-bar-style", content: "black-translucent" },
        { name: "apple-mobile-web-app-title", content: "Collection" },
      ],
      link: [
        { rel: "apple-touch-icon", href: "/apple-touch-icon-180x180.png" },
        // @vite-pwa should inject this, but in dev mode it's intermittent.
        // Being explicit ensures installability works in `docker compose up`.
        { rel: "manifest", href: "/manifest.webmanifest" },
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
