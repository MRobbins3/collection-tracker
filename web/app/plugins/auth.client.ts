/**
 * Hydrate auth state AFTER hydration finishes. Session lives in an HTTP-only
 * cookie that only the browser can see, so SSR always renders the anonymous
 * state; deferring the /me fetch means the client hydrates against the same
 * anonymous state (no hydration mismatch). The signed-in state pops in as a
 * normal post-hydration reactivity update.
 *
 * onNuxtReady, not the `app:mounted` hook: app:mounted fires before an async
 * page (top-level `await useAsyncData`, e.g. /my) has finished hydrating
 * inside <Suspense>. If /me resolved in that window, the header would
 * hydrate signed-in against anonymous SSR markup, and Vue does not patch
 * attribute mismatches during hydration (the menu kept the anonymous
 * element's attributes). onNuxtReady runs once, after hydration completes.
 *
 * The provider list (Google / test-user sign-in) loads the same way, so the
 * sign-in buttons render the Google-only default until it arrives.
 */
export default defineNuxtPlugin(() => {
  onNuxtReady(() => {
    const { refresh, refreshProviders } = useAuth();
    void refresh();
    void refreshProviders();
  });
});
