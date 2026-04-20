/**
 * Sync the useTheme state with what the pre-hydration inline script already
 * applied to <html>. Runs after mount to avoid triggering hydration mismatches
 * on template content that reads `preference` / `effective`.
 */
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook("app:mounted", () => {
    const { hydrateFromBrowser } = useTheme();
    hydrateFromBrowser();
  });
});
