import type { OverviewResponse, OverviewSort } from "~/types/api";

export interface OverviewParams {
  sort: OverviewSort;
  categories: string[];
}

/** One page of items per request; "Show more" asks for the next page. */
export const OVERVIEW_PAGE_SIZE = 50;

const emptyOverview = (): OverviewResponse => ({
  categories: [],
  show_empty_categories: true,
  items: [],
  total: 0,
  has_more: false,
});

function overviewQueryParams(p: OverviewParams, limit: number, offset = 0) {
  return {
    sort: p.sort,
    category: p.categories.length > 0 ? p.categories.join(",") : undefined,
    limit,
    offset: offset > 0 ? offset : undefined,
  };
}

/**
 * Your overview: categories (with counts and pins), the show-empty setting,
 * and the first page of your items for the given sort and filter. Refetches
 * when the params change. Signed-in only, client-only (signedInFetchOptions).
 */
export function useMyOverview(params: Readonly<Ref<OverviewParams>>) {
  const api = useApi();
  const { isSignedIn, server } = signedInFetchOptions();
  // Watch a string so a recomputed-but-equal params object doesn't refetch.
  const paramsKey = computed(() => JSON.stringify(params.value));
  return useAsyncData<OverviewResponse>(
    "me:overview",
    async () =>
      isSignedIn.value
        ? api.get<OverviewResponse>("/me/overview", overviewQueryParams(params.value, OVERVIEW_PAGE_SIZE))
        : emptyOverview(),
    { server, watch: [isSignedIn, paramsKey], default: emptyOverview },
  );
}

/** Just your overview categories and setting (no items), e.g. for the category pages. */
export function useMyOverviewCategories() {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions();
  return useAsyncData<OverviewResponse>(
    "me:overview:categories",
    async () =>
      isSignedIn.value
        ? api.get<OverviewResponse>("/me/overview", { limit: 0 })
        : emptyOverview(),
    { server, watch, default: emptyOverview },
  );
}

export function useOverviewActions() {
  const api = useApi();
  const { publicBaseURL } = api;

  /** Add a category to your overview (idempotent). */
  async function pin(slug: string): Promise<void> {
    await $fetch(`${publicBaseURL}/me/pinned-categories`, {
      method: "POST",
      credentials: "include",
      body: { category_slug: slug },
    });
  }

  /** Remove a category you added (idempotent). */
  async function unpin(slug: string): Promise<void> {
    await $fetch(`${publicBaseURL}/me/pinned-categories/${encodeURIComponent(slug)}`, {
      method: "DELETE",
      credentials: "include",
    });
  }

  async function setShowEmpty(show: boolean): Promise<boolean> {
    const res = await $fetch<{ show_empty_categories: boolean }>(`${publicBaseURL}/me/overview-settings`, {
      method: "PATCH",
      credentials: "include",
      body: { show_empty_categories: show },
    });
    return res.show_empty_categories;
  }

  /** The page of items after the first `offset`, for "Show more". */
  async function loadMore(params: OverviewParams, offset: number): Promise<OverviewResponse> {
    return await api.get<OverviewResponse>("/me/overview", overviewQueryParams(params, OVERVIEW_PAGE_SIZE, offset));
  }

  return { pin, unpin, setShowEmpty, loadMore };
}
