import type {
  CatalogEntry,
  CatalogOwnershipResponse,
  CatalogPage,
  CatalogSuggestionsResponse,
  Item,
} from "~/types/api";

/** Entries per page on a category's catalog list. */
export const CATALOG_PAGE_SIZE = 24;
/** Matches shown in the add-item autocomplete. */
export const CATALOG_AUTOCOMPLETE_LIMIT = 8;

const emptyPage = (): CatalogPage => ({ entries: [], total: 0, has_more: false });

/**
 * First page of a category's approved catalog entries (public, so it renders
 * on the server too). `query` should already be normalized and debounced;
 * further pages come from useCatalogActions().fetchPage via useShowMore.
 */
export function useCatalogPage(slug: string, query: Readonly<Ref<string>>) {
  const api = useApi();
  return useAsyncData<CatalogPage>(
    `catalog:${slug}`,
    () =>
      api.get<CatalogPage>("/catalog/entries", {
        category_slug: slug,
        q: query.value || undefined,
        limit: CATALOG_PAGE_SIZE,
      }),
    { watch: [query], default: emptyPage },
  );
}

/** One approved catalog entry (public). Unknown or unapproved entries are an error (404). */
export function useCatalogEntry(id: string) {
  const api = useApi();
  return useAsyncData<CatalogEntry>(`catalog-entry:${id}`, () => api.get<CatalogEntry>(`/catalog/entries/${id}`));
}

/**
 * Which of a category's catalog entries the signed-in user already has
 * linked items for, for the "In your collection" marker. Anonymous visitors
 * get an empty list without a request (see signedInFetchOptions).
 */
export function useCatalogOwnership(slug: string) {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions();
  return useAsyncData<CatalogOwnershipResponse>(
    `me:catalog-ownership:${slug}`,
    async () =>
      isSignedIn.value
        ? api.get<CatalogOwnershipResponse>("/me/catalog-ownership", { category_slug: slug })
        : { owned: [] },
    { server, watch, default: () => ({ owned: [] }) },
  );
}

/** Catalog search for the add-item autocomplete: the best few matches in a category. */
export function useCatalogSearch() {
  const api = useApi();
  async function search(categorySlug: string, q: string): Promise<CatalogPage> {
    return await api.get<CatalogPage>("/catalog/entries", {
      category_slug: categorySlug,
      q: q || undefined,
      limit: CATALOG_AUTOCOMPLETE_LIMIT,
    });
  }
  return { search };
}

/** Imperative catalog calls: paging, and suggesting/linking/unlinking for your items. */
export function useCatalogActions() {
  const api = useApi();
  const { publicBaseURL } = api;
  const itemURL = (collectionId: string, itemId: string) =>
    `${publicBaseURL}/me/collections/${collectionId}/items/${itemId}`;

  async function fetchPage(slug: string, q: string, offset: number): Promise<CatalogPage> {
    return await api.get<CatalogPage>("/catalog/entries", {
      category_slug: slug,
      q: q || undefined,
      limit: CATALOG_PAGE_SIZE,
      offset,
    });
  }

  async function suggestions(collectionId: string, itemId: string): Promise<CatalogSuggestionsResponse> {
    return await $fetch<CatalogSuggestionsResponse>(`${itemURL(collectionId, itemId)}/catalog-suggestions`, {
      credentials: "include",
    });
  }

  async function link(collectionId: string, itemId: string, entryId: string): Promise<Item> {
    return await $fetch<Item>(`${itemURL(collectionId, itemId)}/catalog-entry`, {
      method: "PUT",
      credentials: "include",
      body: { catalog_entry_id: entryId },
    });
  }

  async function unlink(collectionId: string, itemId: string): Promise<Item> {
    return await $fetch<Item>(`${itemURL(collectionId, itemId)}/catalog-entry`, {
      method: "DELETE",
      credentials: "include",
    });
  }

  /** Add a catalog entry to one of your collections: an item linked to it, details filled in. */
  async function addEntry(collectionId: string, entry: CatalogEntry): Promise<Item> {
    return await $fetch<Item>(`${publicBaseURL}/me/collections/${collectionId}/items`, {
      method: "POST",
      credentials: "include",
      body: { name: entry.name, quantity: 1, attributes: entry.attributes ?? {}, catalog_entry_id: entry.id },
    });
  }

  return { fetchPage, suggestions, link, unlink, addEntry };
}
