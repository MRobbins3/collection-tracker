/**
 * Hydrate auth state on first client render. Session lives in an HTTP-only
 * cookie that the browser — but not SSR — can see, so we only hit /me on the
 * client. Failure is silent (user stays anonymous).
 */
export default defineNuxtPlugin(async () => {
  const { refresh } = useAuth();
  await refresh();
});
