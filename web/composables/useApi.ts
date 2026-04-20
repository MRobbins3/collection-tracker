/**
 * Minimal $fetch wrapper that prefixes requests with the configured API base
 * and sends credentials. Used by data composables; keeps pages framework-agnostic.
 */
export function useApi() {
  const config = useRuntimeConfig();
  // On the server (Nitro/SSR) we reach the API via its internal hostname;
  // in the browser we use the public URL the user's phone can reach.
  const base = (import.meta.server ? config.apiBase : config.public.apiBase) as string;

  async function get<T>(path: string, query?: Record<string, string | number | undefined>): Promise<T> {
    return await $fetch<T>(path, {
      baseURL: base,
      credentials: "include",
      query,
    });
  }

  return { get, baseURL: base };
}
