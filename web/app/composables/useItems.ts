import type { Item, ItemsResponse, ItemSearchResponse } from "~/types/api";

export function useCollectionItems(collectionId: string) {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions(); // see useCollections.ts
  return useAsyncData<ItemsResponse>(
    `collection:${collectionId}:items`,
    async () =>
      isSignedIn.value ? api.get<ItemsResponse>(`/me/collections/${collectionId}/items`) : { items: [] },
    { server, watch, default: () => ({ items: [] }) },
  );
}

/**
 * Search across the signed-in user's own items (GET /me/items). `query`
 * should already be normalized and debounced; a blank query or an
 * anonymous visitor skips the request and yields no results.
 */
export function useMyItemSearch(query: Readonly<Ref<string>>) {
  const api = useApi();
  const { isSignedIn, server } = signedInFetchOptions();
  const empty = (): ItemSearchResponse => ({ items: [], has_more: false });
  return useAsyncData<ItemSearchResponse>(
    "me:items:search",
    async () =>
      isSignedIn.value && query.value ? api.get<ItemSearchResponse>("/me/items", { q: query.value }) : empty(),
    { server, watch: [isSignedIn, query], default: empty },
  );
}

export interface ItemWriteInput {
  name: string;
  quantity: number;
  condition?: string;
  attributes?: Record<string, unknown>;
  /** Links a new item to a catalog entry (create only; the API ignores it on update). */
  catalog_entry_id?: string;
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
