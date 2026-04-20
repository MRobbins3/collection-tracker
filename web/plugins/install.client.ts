/**
 * Wire the install composable to the window beforeinstallprompt event on
 * client mount. Runs after app-mounted for the same reason auth does —
 * avoiding any hydration surprises.
 */
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook("app:mounted", () => {
    const { hydrate } = useInstall();
    hydrate();
  });
});
