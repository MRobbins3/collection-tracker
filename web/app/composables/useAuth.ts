import type { AuthProviders, DevPersona, User } from "~/types/api";

/**
 * useAuth exposes the current user (null when anonymous), which sign-in
 * providers the API offers, and login/logout actions. State lives in
 * useState shared across the whole app; refresh() re-fetches /me and is safe
 * to call multiple times.
 *
 * `providers` stays null until the auth plugin loads it after app:mounted,
 * so SSR and the first client render agree (hydration parity). Components
 * treat null as "Google only", the pre-dev-auth default.
 */
export function useAuth() {
  const user = useState<User | null>("auth:user", () => null);
  const providers = useState<AuthProviders | null>("auth:providers", () => null);
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

  async function refreshProviders(): Promise<void> {
    try {
      providers.value = await api.get<AuthProviders>("/auth/providers");
    } catch {
      // Leave null: components fall back to the Google-only default.
    }
  }

  // URLs that end up in the browser (href, window.location, fetch at click
  // time) must use the public base URL so they're identical during SSR and
  // client hydration.
  function loginURL(): string {
    return `${api.publicBaseURL}/auth/google/start`;
  }

  /** Local/CI only: the seeded test users the dev sign-in picker offers. */
  async function listPersonas(): Promise<DevPersona[]> {
    const { personas } = await api.get<{ personas: DevPersona[] }>("/auth/dev/personas");
    return personas;
  }

  /**
   * Local/CI only: sign in as a seeded test user. The API sets the same
   * session cookie Google sign-in does. There's no page reload (unlike the
   * Google redirect), so re-run every page's data fetch for the new user.
   */
  async function devLogin(handle: string): Promise<void> {
    // An object body makes $fetch send Content-Type: application/json, which
    // the API requires (blocks cross-site form posts; see ADR 0008).
    const res = await $fetch<{ user: User }>(`${api.publicBaseURL}/auth/dev/login`, {
      method: "POST",
      credentials: "include",
      body: { persona: handle },
    });
    user.value = res.user;
    await refreshNuxtData();
  }

  async function logout(): Promise<void> {
    await $fetch(`${api.publicBaseURL}/auth/logout`, {
      method: "POST",
      credentials: "include",
    });
    user.value = null;
  }

  const isSignedIn = computed(() => user.value !== null);

  return {
    user,
    providers,
    isSignedIn,
    loading,
    refresh,
    refreshProviders,
    loginURL,
    listPersonas,
    devLogin,
    logout,
  };
}
