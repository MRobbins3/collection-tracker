<script setup lang="ts">
import type { CategoriesResponse } from "~/types/api";
import { normalizeSearchQuery, SEARCH_DEBOUNCE_MS } from "~/utils/search";
import {
  overviewQuery,
  parseOverviewQuery,
  visibleCategories,
  type OverviewState,
} from "~/utils/overview";

const { isSignedIn, user } = useAuth();
const { data, refresh: refreshCollections } = useMyCollections();
const { create, remove } = useMyCollectionsActions();
const { pin, unpin, setShowEmpty, loadMore } = useOverviewActions();
const api = useApi();
const hydrated = useHydrated();

const { data: categoriesData } = await useAsyncData("categories:all", () =>
  api.get<CategoriesResponse>("/categories"),
);

useHead({ title: "My collections · Collection Tracker" });

const route = useRoute();
const router = useRouter();

// ---- Overview state ---------------------------------------------------------
// The URL is the source of truth for view / sort / category filter, so the
// page is shareable and Back undoes a filter change (each change pushes a
// history entry). See utils/overview.ts.
const state = computed(() => parseOverviewQuery(route.query));
function setState(patch: Partial<OverviewState>) {
  void router.push({ query: overviewQuery({ ...state.value, ...patch }, route.query) });
}
const params = computed(() => ({ sort: state.value.sort, categories: state.value.categories }));

const {
  data: overview,
  status: overviewStatus,
  error: overviewError,
  refresh: refreshOverview,
} = useMyOverview(params);
const overviewPending = computed(() => overviewStatus.value === "pending");
const loadedOnce = ref(false);
watch(overviewStatus, (s) => {
  if (s === "success") loadedOnce.value = true;
});

const showEmpty = computed(() => overview.value.show_empty_categories);
const visible = computed(() => visibleCategories(overview.value.categories, showEmpty.value));
const hiddenCount = computed(() => overview.value.categories.length - visible.value.length);
// Chips: the visible categories, plus any selected one hidden by the
// show-empty setting so it can still be cleared.
const chipCategories = computed(() =>
  overview.value.categories.filter((c) => visible.value.includes(c) || state.value.categories.includes(c.slug)),
);
const groups = computed(() =>
  state.value.categories.length === 0
    ? visible.value
    : visible.value.filter((c) => state.value.categories.includes(c.slug)),
);

// "Show more" pages are appended locally and reset when the first page's
// items change; a page still loading when that happens is dropped.
const {
  items,
  hasMore,
  loadingMore,
  error: loadMoreError,
  loadMore: onLoadMore,
} = useShowMore(overview, (offset) => loadMore(params.value, offset));

const actionError = ref<string | null>(null);
const shownError = computed(
  () => actionError.value ?? (loadMoreError.value ? "Couldn’t load more items. Please try again." : null),
);

async function onShowEmpty(show: boolean) {
  actionError.value = null;
  const prev = overview.value.show_empty_categories;
  overview.value = { ...overview.value, show_empty_categories: show };
  try {
    await setShowEmpty(show);
  } catch (e) {
    console.error(e);
    overview.value = { ...overview.value, show_empty_categories: prev };
    actionError.value = "Couldn’t save that setting. Please try again.";
  }
}

// ---- Add a category ---------------------------------------------------------
const adding = ref(false);
const busySlug = ref<string | null>(null);
const panelError = ref<string | null>(null);

async function onPin(slug: string) {
  busySlug.value = slug;
  panelError.value = null;
  try {
    await pin(slug);
    await refreshOverview();
  } catch (e) {
    console.error(e);
    panelError.value = "Couldn’t add that category. Please try again.";
  } finally {
    busySlug.value = null;
  }
}

async function onUnpin(slug: string) {
  busySlug.value = slug;
  panelError.value = null;
  actionError.value = null;
  try {
    await unpin(slug);
    await refreshOverview();
  } catch (e) {
    console.error(e);
    const msg = "Couldn’t remove that category. Please try again.";
    if (adding.value) panelError.value = msg;
    else actionError.value = msg;
  } finally {
    busySlug.value = null;
  }
}

async function onStartCollection(slug: string, name: string) {
  busySlug.value = slug;
  actionError.value = null;
  try {
    // Named after the category; rename it on the collection page.
    const created = await create({ categorySlug: slug, name });
    await navigateTo(`/my/${created.id}`);
  } catch (e) {
    console.error(e);
    actionError.value = "Couldn’t start that collection. Please try again.";
  } finally {
    busySlug.value = null;
  }
}

// ---- Item search ------------------------------------------------------------
// Item search lives inline on this page. The query is kept in ?q= so Back
// from a result returns to the same results (and a reload keeps them).
const initialQuery = route.query.q;
const searchText = ref(typeof initialQuery === "string" ? initialQuery : "");
const searchTerm = computed(() => normalizeSearchQuery(searchText.value));
const searching = computed(() => searchTerm.value !== "");
const hasCollections = computed(() => (data.value?.collections.length ?? 0) > 0);
const urlTerm = useDebouncedRef(searchTerm, SEARCH_DEBOUNCE_MS);
watch(urlTerm, (q) => {
  const { q: _old, ...rest } = route.query;
  void router.replace({ query: q ? { ...rest, q } : rest });
});
// The page component is reused across /my?q=… → /my (e.g. the header's
// "My collections" link), so follow ?q= when it changes from outside. Our
// own replace above writes urlTerm, so comparing against it ignores those.
watch(
  () => route.query.q,
  (q) => {
    const next = typeof q === "string" ? q : "";
    if (normalizeSearchQuery(next) !== urlTerm.value) searchText.value = next;
  },
);

// ---- Collections ------------------------------------------------------------
async function onCreated() {
  await Promise.all([refreshCollections(), refreshOverview()]);
}

async function onDelete(id: string, name: string) {
  if (!confirm(`Delete “${name}”? This removes all items inside it.`)) return;
  await remove(id);
  await Promise.all([refreshCollections(), refreshOverview()]);
}
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-xl font-bold tracking-tight text-slate-900 dark:text-slate-50">
        My collections
      </h1>
      <p v-if="isSignedIn" class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Hey {{ user?.display_name }} — here’s everything you’re tracking.
      </p>
      <p v-else class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Sign in to start tracking what you own.
      </p>
    </div>

    <SignInPrompt
      v-if="!isSignedIn"
      message="Sign in to create collections and add items."
    />

    <template v-else>
      <ItemSearch v-if="hasCollections || searching" v-model="searchText" />

      <template v-if="!searching">
        <OverviewControls
          v-if="loadedOnce"
          :view="state.view"
          :categories="chipCategories"
          :selected="state.categories"
          :show-empty="showEmpty"
          :disabled="!hydrated"
          @update:view="(view) => setState({ view })"
          @update:selected="(categories) => setState({ categories })"
          @update:show-empty="onShowEmpty"
          @add-category="adding = !adding"
        />

        <AddCategoryPanel
          v-if="adding && categoriesData"
          :categories="categoriesData.categories"
          :overview="overview.categories"
          :busy-slug="busySlug"
          :error="panelError"
          @pin="onPin"
          @unpin="onUnpin"
          @close="adding = false"
        />

        <p
          v-if="shownError"
          role="alert"
          class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
          data-testid="overview-action-error"
        >
          {{ shownError }}
        </p>

        <template v-if="state.view === 'items'">
          <OverviewItemList
            :items="items"
            :total="overview.total"
            :has-more="hasMore"
            :sort="state.sort"
            :pending="overviewPending || !loadedOnce"
            :loading-more="loadingMore"
            :error="!!overviewError"
            :filtered="state.categories.length > 0"
            :disabled="!hydrated"
            @update:sort="(sort) => setState({ sort })"
            @load-more="onLoadMore"
            @clear-filter="setState({ categories: [] })"
            @retry="refreshOverview()"
          />
        </template>

        <template v-else>
          <div v-if="!loadedOnce && !overviewError" class="text-sm text-slate-500 dark:text-slate-400">
            Loading your collections…
          </div>

          <div
            v-else-if="overviewError"
            role="alert"
            class="flex items-center justify-between gap-3 rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
            data-testid="overview-error"
          >
            <span>Couldn’t load your collections. Please try again.</span>
            <button
              type="button"
              class="shrink-0 rounded-md border border-rose-200 px-2 py-1 text-xs font-medium hover:bg-rose-100 dark:border-rose-800 dark:hover:bg-rose-900"
              @click="refreshOverview()"
            >
              Try again
            </button>
          </div>

          <template v-else-if="overview.categories.length > 0">
            <CategoryGroups
              v-if="groups.length > 0"
              :categories="groups"
              :collections="data?.collections ?? []"
              :busy-slug="busySlug"
              @delete-collection="onDelete"
              @start-collection="onStartCollection"
              @unpin="onUnpin"
            />
            <p
              v-else
              class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
              data-testid="overview-groups-empty"
            >
              Nothing to show for the selected categories.
              <button
                type="button"
                class="mt-2 block w-full font-medium text-slate-700 underline hover:text-slate-900 dark:text-slate-200 dark:hover:text-white"
                data-testid="overview-groups-clear-filter"
                @click="setState({ categories: [] })"
              >
                Clear filter
              </button>
            </p>

            <p
              v-if="hiddenCount > 0"
              class="text-center text-sm text-slate-500 dark:text-slate-400"
              data-testid="hidden-empty-note"
            >
              {{ hiddenCount }} empty {{ hiddenCount === 1 ? "category" : "categories" }} hidden.
              <button
                type="button"
                class="font-medium text-slate-700 underline hover:text-slate-900 dark:text-slate-200 dark:hover:text-white"
                @click="onShowEmpty(true)"
              >
                Show {{ hiddenCount === 1 ? "it" : "them" }}
              </button>
            </p>
          </template>

          <div
            v-else
            class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            data-testid="my-collections-empty"
          >
            You don’t have any collections yet. Start one below, or add a category to see it here.
          </div>

          <NewCollectionForm
            v-if="categoriesData && categoriesData.categories.length > 0"
            :categories="categoriesData.categories"
            @created="onCreated"
          />
        </template>
      </template>
    </template>
  </section>
</template>
