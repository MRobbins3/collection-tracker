import type { User } from "~/types/api";

/**
 * useAuth exposes the current user (null when anonymous) and login/logout
 * actions. Session state lives in a useState shared across the whole app;
 * refresh() re-fetches /me and is safe to call multiple times.
 */
export function useAuth() {
  const user = useState<User | null>("auth:user", () => null);
  const loading = useState<boolean>("auth:loading", () => false);
  const api = useApi();

  async function refresh(): Promise<void> {
    loading.value = true;
    try {
      // /me always returns 200 with { user: User | null } — anonymous is not
      // an error state, it's just the default.
      const { user: fresh } = await api.get<{ user: User | null }>("/me");
      user.value = fresh;
    } catch {
      user.value = null;
    } finally {
      loading.value = false;
    }
  }

  // URLs that end up in the browser (href, window.location, fetch at click
  // time) must use the public base URL so they're identical during SSR and
  // client hydration.
  function loginURL(): string {
    return `${api.publicBaseURL}/auth/google/start`;
  }

  async function logout(): Promise<void> {
    await $fetch(`${api.publicBaseURL}/auth/logout`, {
      method: "POST",
      credentials: "include",
    });
    user.value = null;
  }

  const isSignedIn = computed(() => user.value !== null);

  return { user, isSignedIn, loading, refresh, loginURL, logout };
}
