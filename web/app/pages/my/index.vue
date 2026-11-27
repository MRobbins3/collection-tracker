<script setup lang="ts">
import type { CategoriesResponse } from "~/types/api";
import { normalizeSearchQuery, SEARCH_DEBOUNCE_MS } from "~/utils/search";

const { isSignedIn, user } = useAuth();
const { data, refresh: refreshCollections, pending } = useMyCollections();
const { remove } = useMyCollectionsActions();
const api = useApi();

const { data: categoriesData } = await useAsyncData("categories:all", () =>
  api.get<CategoriesResponse>("/categories"),
);

useHead({ title: "My collections · Collection Tracker" });

// Item search lives inline on this page. The query is kept in ?q= so Back
// from a result returns to the same results (and a reload keeps them).
const route = useRoute();
const router = useRouter();
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

async function onCreated() {
  await refreshCollections();
}

async function onDelete(id: string, name: string) {
  if (!confirm(`Delete “${name}”? This removes all items inside it.`)) return;
  await remove(id);
  await refreshCollections();
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
        <NewCollectionForm
          v-if="categoriesData && categoriesData.categories.length > 0"
          :categories="categoriesData.categories"
          @created="onCreated"
        />

        <div v-if="pending && !data" class="text-sm text-slate-500 dark:text-slate-400">
          Loading your collections…
        </div>

        <ul
          v-else-if="data && data.collections.length > 0"
          class="space-y-2"
          data-testid="my-collections-list"
        >
          <li
            v-for="c in data.collections"
            :key="c.id"
            class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
            :data-testid="`my-collection-${c.id}`"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0 flex-1">
                <NuxtLink :to="`/my/${c.id}`" class="block">
                  <h3
                    class="truncate text-base font-semibold text-slate-900 dark:text-slate-50"
                  >
                    {{ c.name }}
                  </h3>
                  <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
                    {{ c.category_name }} · {{ c.item_count }} item{{ c.item_count === 1 ? "" : "s" }}
                  </p>
                </NuxtLink>
              </div>
              <button
                type="button"
                class="shrink-0 rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                :data-testid="`delete-collection-${c.id}`"
                @click="onDelete(c.id, c.name)"
              >
                Delete
              </button>
            </div>
          </li>
        </ul>

        <div
          v-else
          class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
          data-testid="my-collections-empty"
        >
          You don’t have any collections yet. Pick a category above and start your first one.
        </div>
      </template>
    </template>
  </section>
</template>
