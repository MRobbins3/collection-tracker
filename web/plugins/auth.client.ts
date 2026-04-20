/**
 * Hydrate auth state AFTER the app mounts. Session lives in an HTTP-only
 * cookie that only the browser can see, so SSR always renders the anonymous
 * state; deferring the /me fetch until `app:mounted` means the first client
 * render matches the server HTML (no hydration mismatch). The signed-in
 * state pops in as a normal post-hydration reactivity update.
 */
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook("app:mounted", () => {
    const { refresh } = useAuth();
    void refresh();
  });
});
