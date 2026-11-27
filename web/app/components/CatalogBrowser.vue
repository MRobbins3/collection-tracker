<script setup lang="ts">
import { computed, ref, useId } from "vue";
import { useDebouncedRef } from "~/composables/useDebouncedRef";
import { useHydrated } from "~/composables/useHydrated";
import { useShowMore } from "~/composables/useShowMore";
import { catalogEntryPath, identifierTitle, midSentence } from "~/utils/catalog";
import { normalizeSearchQuery, SEARCH_DEBOUNCE_MS } from "~/utils/search";
import type { CatalogEntry, Category } from "~/types/api";

/**
 * A category's global catalog (ADR 0004): approved entries with search and
 * "Show more" paging. Anyone can browse; signed-in visitors also see which
 * entries are already in their collection.
 */
const props = defineProps<{ category: Category }>();

const inputId = useId();
// Server-rendered; the box and "Show more" only work once hydrated (see useHydrated).
const hydrated = useHydrated();
const text = ref("");
const normalized = computed(() => normalizeSearchQuery(text.value));
const term = useDebouncedRef(normalized, SEARCH_DEBOUNCE_MS);

const { data, pending, error, refresh } = useCatalogPage(props.category.slug, term);
const { data: ownership } = useCatalogOwnership(props.category.slug);
const { fetchPage } = useCatalogActions();

const firstPage = computed(() => ({
  items: data.value?.entries ?? [],
  has_more: data.value?.has_more ?? false,
}));
// The term is read when "Show more" is clicked; a new search resets the
// list (and drops any page still in flight) via useShowMore's generation.
const more = useShowMore<CatalogEntry>(firstPage, async (offset) => {
  const page = await fetchPage(props.category.slug, term.value, offset);
  return { items: page.entries, has_more: page.has_more };
});

const idTitle = computed(() => identifierTitle(props.category));
const ownedEntries = computed(() => new Set((ownership.value?.owned ?? []).map((o) => o.catalog_entry_id)));
const total = computed(() => data.value?.total ?? 0);
const searching = computed(() => normalized.value !== term.value || pending.value);

const status = computed(() => {
  if (error.value || searching.value) return "";
  const n = total.value;
  if (term.value) return n === 0 ? "No matches" : `${n} match${n === 1 ? "" : "es"}`;
  return `${n} entr${n === 1 ? "y" : "ies"}`;
});
</script>

<template>
  <section
    class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="catalog-browser"
    aria-labelledby="catalog-heading"
  >
    <div class="flex items-baseline justify-between gap-3">
      <h2 id="catalog-heading" class="text-base font-semibold text-slate-900 dark:text-slate-100">
        {{ category.name }} catalog
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400" role="status" aria-live="polite" data-testid="catalog-status">
        {{ status }}
      </p>
    </div>

    <form role="search" class="relative" @submit.prevent>
      <label :for="inputId" class="sr-only">Search the {{ category.name }} catalog</label>
      <Icon
        name="lucide:search"
        class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400 dark:text-slate-500"
        aria-hidden="true"
      />
      <input
        :id="inputId"
        v-model="text"
        :disabled="!hydrated"
        type="search"
        inputmode="search"
        enterkeyhint="search"
        autocomplete="off"
        :placeholder="idTitle ? `Search by name or ${midSentence(idTitle)}…` : 'Search by name…'"
        class="block w-full rounded-lg border-slate-300 bg-white py-2 pl-9 pr-3 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        data-testid="catalog-search"
      >
    </form>

    <div
      v-if="error"
      role="alert"
      class="flex items-center justify-between gap-3 rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      data-testid="catalog-error"
    >
      <span>Couldn’t load the catalog. Please try again.</span>
      <button
        type="button"
        class="shrink-0 rounded-md border border-rose-200 px-2 py-1 text-xs font-medium hover:bg-rose-100 dark:border-rose-800 dark:hover:bg-rose-900"
        @click="refresh()"
      >
        Try again
      </button>
    </div>

    <div
      v-else-if="searching && more.items.value.length === 0"
      class="text-sm text-slate-500 dark:text-slate-400"
      data-testid="catalog-loading"
    >
      Loading…
    </div>

    <ul
      v-else-if="more.items.value.length > 0"
      class="divide-y divide-slate-100 transition-opacity dark:divide-slate-800"
      :class="{ 'opacity-60': searching }"
      :aria-busy="searching"
      data-testid="catalog-list"
    >
      <li v-for="entry in more.items.value" :key="entry.id" data-testid="catalog-entry">
        <NuxtLink
          :to="catalogEntryPath(category.slug, entry.id)"
          class="-mx-2 flex items-start justify-between gap-3 rounded-lg px-2 py-3 hover:bg-slate-50 dark:hover:bg-slate-800"
        >
          <div class="min-w-0">
            <h3 class="truncate text-sm font-semibold text-slate-900 dark:text-slate-50">{{ entry.name }}</h3>
            <p v-if="idTitle && entry.identifier" class="text-xs text-slate-500 dark:text-slate-400">
              {{ idTitle }} {{ entry.identifier }}
            </p>
            <p v-if="entry.description" class="mt-0.5 line-clamp-2 text-xs text-slate-600 dark:text-slate-300">
              {{ entry.description }}
            </p>
          </div>
          <span
            v-if="ownedEntries.has(entry.id)"
            class="inline-flex shrink-0 items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-800 ring-1 ring-emerald-200 dark:bg-emerald-950 dark:text-emerald-200 dark:ring-emerald-800"
            data-testid="catalog-entry-owned"
          >
            <Icon name="lucide:check" class="h-3 w-3" aria-hidden="true" />
            In your collection
          </span>
        </NuxtLink>
      </li>
    </ul>

    <div
      v-else
      class="rounded-lg bg-slate-100 p-4 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      data-testid="catalog-empty"
    >
      <template v-if="term">Nothing in the catalog matches “{{ term }}”.</template>
      <template v-else>No {{ category.name }} in the catalog yet. You can still add your own items to a collection.</template>
    </div>

    <p v-if="more.error.value" role="alert" class="text-sm text-rose-700 dark:text-rose-300">
      Couldn’t load more entries. Please try again.
    </p>
    <button
      v-if="!error && more.hasMore.value"
      type="button"
      :disabled="!hydrated || more.loadingMore.value"
      class="inline-flex h-10 w-full items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
      data-testid="catalog-show-more"
      @click="more.loadMore()"
    >
      {{ more.loadingMore.value ? "Loading…" : "Show more" }}
    </button>
  </section>
</template>
