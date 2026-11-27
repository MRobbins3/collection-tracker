import type { Item, ItemsResponse, CatalogSearchResponse } from "~/types/api";

export function useCollectionItems(collectionId: string) {
  const api = useApi();
  return useAsyncData<ItemsResponse>(
    `collection:${collectionId}:items`,
    () => api.get<ItemsResponse>(`/me/collections/${collectionId}/items`),
    { default: () => ({ items: [] }) },
  );
}

export interface ItemWriteInput {
  name: string;
  quantity: number;
  condition?: string;
  attributes?: Record<string, unknown>;
}

export function useItemsActions(collectionId: string) {
  const { publicBaseURL } = useApi();

  async function create(input: ItemWriteInput): Promise<Item> {
    return await $fetch<Item>(`${publicBaseURL}/me/collections/${collectionId}/items`, {
      method: "POST",
      credentials: "include",
      body: input,
    });
  }
  async function update(id: string, input: ItemWriteInput): Promise<Item> {
    return await $fetch<Item>(`${publicBaseURL}/me/collections/${collectionId}/items/${id}`, {
      method: "PATCH",
      credentials: "include",
      body: input,
    });
  }
  async function remove(id: string): Promise<void> {
    await $fetch(`${publicBaseURL}/me/collections/${collectionId}/items/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
  }
  return { create, update, remove };
}

export function useCatalogSearch() {
  const api = useApi();
  async function search(categorySlug: string, q: string): Promise<CatalogSearchResponse> {
    return await api.get<CatalogSearchResponse>("/catalog/entries", {
      category_slug: categorySlug,
      q: q || undefined,
    });
  }
  return { search };
}
