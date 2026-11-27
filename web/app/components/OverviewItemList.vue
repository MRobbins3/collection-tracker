<script setup lang="ts">
import { computed, useId } from "vue";
import type { OverviewItem, OverviewSort } from "~/types/api";
import { SORT_OPTIONS } from "~/utils/overview";

/**
 * The overview's "All items" view: every item across your collections, with
 * a sort control and "Show more" paging. Presentational; the page fetches.
 */
const props = defineProps<{
  items: OverviewItem[];
  total: number;
  hasMore: boolean;
  sort: OverviewSort;
  pending: boolean;
  loadingMore: boolean;
  error: boolean;
  /** A category filter is active (changes the empty-state copy). */
  filtered: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  (e: "update:sort", value: OverviewSort): void;
  (e: "loadMore" | "retry" | "clearFilter"): void;
}>();

const sortId = useId();

const countText = computed(() => {
  const unit = props.total === 1 ? "item" : "items";
  if (props.items.length < props.total) return `Showing ${props.items.length} of ${props.total} ${unit}`;
  return `${props.total} ${unit}`;
});

// Items are fetched client-only, so formatting in the viewer's locale and
// time zone can't cause a hydration mismatch.
const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
function added(iso: string): string {
  return dateFormat.format(new Date(iso));
}

function onSort(e: Event) {
  emit("update:sort", (e.target as HTMLSelectElement).value as OverviewSort);
}
</script>

<template>
  <div class="space-y-3" data-testid="overview-items">
    <div class="flex items-center justify-between gap-3">
      <p class="text-sm text-slate-600 dark:text-slate-400" aria-live="polite" data-testid="overview-item-count">
        <template v-if="!pending && !error">{{ countText }}</template>
      </p>
      <div class="flex items-center gap-2">
        <label :for="sortId" class="text-sm text-slate-600 dark:text-slate-400">Sort by</label>
        <select
          :id="sortId"
          :value="sort"
          :disabled="disabled"
          class="rounded-md border-slate-300 bg-white py-1.5 pl-2 pr-8 text-sm shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
          data-testid="overview-sort"
          @change="onSort"
        >
          <option v-for="o in SORT_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </div>
    </div>

    <div
      v-if="error"
      role="alert"
      class="flex items-center justify-between gap-3 rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
    >
      <span>Couldn’t load your items. Please try again.</span>
      <button
        type="button"
        class="shrink-0 rounded-md border border-rose-200 px-2 py-1 text-xs font-medium hover:bg-rose-100 dark:border-rose-800 dark:hover:bg-rose-900"
        @click="emit('retry')"
      >
        Try again
      </button>
    </div>

    <div
      v-else-if="pending && items.length === 0"
      class="text-sm text-slate-500 dark:text-slate-400"
      data-testid="overview-items-loading"
    >
      Loading your items…
    </div>

    <template v-else-if="items.length > 0">
      <ul
        class="space-y-2 transition-opacity"
        :class="{ 'opacity-60': pending }"
        :aria-busy="pending"
        data-testid="overview-item-list"
      >
        <li v-for="it in items" :key="it.id" data-testid="overview-item">
          <NuxtLink
            :to="`/my/${it.collection_id}`"
            class="block rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 hover:ring-slate-300 dark:bg-slate-900 dark:ring-slate-800 dark:hover:ring-slate-700"
          >
            <div class="flex items-baseline justify-between gap-3">
              <h3 class="truncate text-base font-semibold text-slate-900 dark:text-slate-50">{{ it.name }}</h3>
              <span v-if="it.quantity > 1" class="shrink-0 text-xs text-slate-500 dark:text-slate-400">
                ×{{ it.quantity }}
              </span>
            </div>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">
              {{ it.collection_name }} · {{ it.category_name }}
            </p>
            <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
              Added <time :datetime="it.created_at">{{ added(it.created_at) }}</time>
            </p>
          </NuxtLink>
        </li>
      </ul>
      <button
        v-if="hasMore"
        type="button"
        :disabled="loadingMore"
        class="block h-10 w-full rounded-lg border border-slate-300 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
        data-testid="overview-load-more"
        @click="emit('loadMore')"
      >
        {{ loadingMore ? "Loading…" : "Show more" }}
      </button>
    </template>

    <div
      v-else
      class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      data-testid="overview-items-empty"
    >
      <template v-if="filtered">
        No items in the selected categories yet.
        <button
          type="button"
          class="mt-2 block w-full font-medium text-slate-700 underline hover:text-slate-900 dark:text-slate-200 dark:hover:text-white"
          data-testid="overview-clear-filter"
          @click="emit('clearFilter')"
        >
          Clear filter
        </button>
      </template>
      <template v-else>You haven’t added any items yet. Open a collection to add your first one.</template>
    </div>
  </div>
</template>
