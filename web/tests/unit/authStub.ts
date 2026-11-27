import { ref, computed } from "vue";
import { vi } from "vitest";
import type { AuthProviders, DevPersona, User } from "~/types/api";

/**
 * Hand-rolled useAuth stub shared by the sign-in component specs. Easier and
 * more honest than importing the Nuxt runtime. Install it with
 * `installAuthStub()` in beforeEach; it becomes the global auto-import.
 */
export function makeAuthStub(init: { user?: User | null; providers?: AuthProviders | null } = {}) {
  const user = ref<User | null>(init.user ?? null);
  const providers = ref<AuthProviders | null>(init.providers ?? null);
  const personas: DevPersona[] = [
    { handle: "liam", display_name: "Liam Johnson", email: "liam.johnson@example.com", collection_count: 2, item_count: 6 },
    { handle: "emma", display_name: "Emma Garcia", email: "emma.garcia@example.com", collection_count: 0, item_count: 0 },
  ];
  return {
    user,
    providers,
    isSignedIn: computed(() => user.value !== null),
    loginURL: () => "http://api.test/auth/google/start",
    logout: vi.fn(async () => {
      user.value = null;
    }),
    listPersonas: vi.fn(async (): Promise<DevPersona[]> => personas),
    devLogin: vi.fn(async (handle: string) => {
      const p = personas.find((x) => x.handle === handle)!;
      user.value = { id: `user-${handle}`, email: p.email, display_name: p.display_name, created_at: "2026-11-27T00:00:00Z" };
    }),
  };
}

export type AuthStub = ReturnType<typeof makeAuthStub>;

export function installAuthStub(stub: AuthStub): void {
  // @ts-expect-error vitest global standing in for the Nuxt auto-import
  globalThis.useAuth = () => stub;
}
