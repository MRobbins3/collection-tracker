import type { Collection, CollectionsResponse } from "~/types/api";

/**
 * Options shared by every signed-in-only (/me/*) data fetch:
 * - server: false — the session cookie never reaches SSR fetches, so a
 *   server-side /me/* call is always a 401 that hydrates as "no data".
 * - watch isSignedIn, and the handlers skip the request while anonymous, so
 *   anonymous visitors never put a 401 in the console (CLAUDE.md), and the
 *   data loads as soon as /me says who you are.
 */
export function signedInFetchOptions() {
  const { isSignedIn } = useAuth();
  return { isSignedIn, server: false, watch: [isSignedIn] };
}

export function useMyCollections() {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions();
  return useAsyncData<CollectionsResponse>(
    "me:collections",
    async () => (isSignedIn.value ? api.get<CollectionsResponse>("/me/collections") : { collections: [] }),
    { server, watch, default: () => ({ collections: [] }) },
  );
}

export function useMyCollection(id: string) {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions();
  return useAsyncData<Collection | null>(
    `me:collection:${id}`,
    async () => (isSignedIn.value ? api.get<Collection>(`/me/collections/${id}`) : null),
    { server, watch },
  );
}

export function useMyCollectionsActions() {
  const { publicBaseURL } = useApi();

  async function create(input: { categorySlug: string; name: string }): Promise<Collection> {
    return await $fetch<Collection>(`${publicBaseURL}/me/collections`, {
      method: "POST",
      credentials: "include",
      body: { category_slug: input.categorySlug, name: input.name },
    });
  }

  async function rename(id: string, name: string): Promise<Collection> {
    return await $fetch<Collection>(`${publicBaseURL}/me/collections/${id}`, {
      method: "PATCH",
      credentials: "include",
      body: { name },
    });
  }

  async function remove(id: string): Promise<void> {
    await $fetch(`${publicBaseURL}/me/collections/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
  }

  return { create, rename, remove };
}
