import type { Collection, CollectionsResponse } from "~/types/api";

export function useMyCollections() {
  const api = useApi();
  return useAsyncData<CollectionsResponse>(
    "me:collections",
    () => api.get<CollectionsResponse>("/me/collections"),
    { default: () => ({ collections: [] }) },
  );
}

export function useMyCollection(id: string) {
  const api = useApi();
  return useAsyncData<Collection>(
    `me:collection:${id}`,
    () => api.get<Collection>(`/me/collections/${id}`),
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
    await $fetch<void>(`${publicBaseURL}/me/collections/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
  }

  return { create, rename, remove };
}
