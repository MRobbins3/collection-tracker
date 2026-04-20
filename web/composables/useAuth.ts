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
      user.value = await api.get<User>("/me");
    } catch {
      user.value = null;
    } finally {
      loading.value = false;
    }
  }

  function loginURL(): string {
    // Full-page navigation — OAuth can't live behind fetch.
    return `${api.baseURL}/auth/google/start`;
  }

  async function logout(): Promise<void> {
    await $fetch(`${api.baseURL}/auth/logout`, { method: "POST", credentials: "include" });
    user.value = null;
  }

  const isSignedIn = computed(() => user.value !== null);

  return { user, isSignedIn, loading, refresh, loginURL, logout };
}
