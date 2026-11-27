<script setup lang="ts">
import type { OverviewCategory } from "~/types/api";
import { toggleSlug, type OverviewView } from "~/utils/overview";

/**
 * The overview's controls: view switch (Collections / All items), category
 * filter chips (multi-select; "All" clears the filter), the "Show empty
 * categories" switch, and the "Add a category" button. Stateless: the page
 * owns the state (in the URL and on the server) and passes it back in.
 */
const props = defineProps<{
  view: OverviewView;
  /** The categories to offer as chips (already filtered by showEmpty). */
  categories: OverviewCategory[];
  selected: string[];
  showEmpty: boolean;
  /** True until the page has hydrated, so SSR markup and first render agree. */
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:view", value: OverviewView): void;
  (e: "update:selected", value: string[]): void;
  (e: "update:showEmpty", value: boolean): void;
  (e: "addCategory"): void;
}>();

const views: { value: OverviewView; label: string }[] = [
  { value: "collections", label: "Collections" },
  { value: "items", label: "All items" },
];

function toggle(slug: string) {
  emit("update:selected", toggleSlug(props.selected, slug));
}

const chipBase =
  "inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full px-3 text-sm font-medium ring-1 transition focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-900 dark:focus-visible:ring-slate-100 disabled:opacity-60";
const chipOn = "bg-slate-900 text-white ring-slate-900 dark:bg-slate-100 dark:text-slate-900 dark:ring-slate-100";
const chipOff =
  "bg-white text-slate-700 ring-slate-300 hover:bg-slate-50 dark:bg-slate-900 dark:text-slate-200 dark:ring-slate-700 dark:hover:bg-slate-800";
</script>

<template>
  <div class="space-y-3" data-testid="overview-controls">
    <div class="flex items-center justify-between gap-3">
      <div
        role="group"
        aria-label="View"
        class="inline-flex rounded-lg bg-slate-100 p-1 dark:bg-slate-800"
      >
        <button
          v-for="v in views"
          :key="v.value"
          type="button"
          :aria-pressed="view === v.value"
          :disabled="disabled"
          class="h-8 rounded-md px-3 text-sm font-medium transition focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-900 dark:focus-visible:ring-slate-100"
          :class="
            view === v.value
              ? 'bg-white text-slate-900 shadow-sm dark:bg-slate-950 dark:text-slate-50'
              : 'text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-slate-50'
          "
          :data-testid="`overview-view-${v.value}`"
          @click="emit('update:view', v.value)"
        >
          {{ v.label }}
        </button>
      </div>

      <button
        type="button"
        :disabled="disabled"
        class="inline-flex h-9 shrink-0 items-center gap-1 rounded-lg border border-slate-300 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
        data-testid="add-category-button"
        @click="emit('addCategory')"
      >
        <Icon name="lucide:plus" class="h-4 w-4" aria-hidden="true" />
        Add a category
      </button>
    </div>

    <div
      v-if="categories.length > 0 || selected.length > 0"
      role="group"
      aria-label="Filter by category"
      class="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1"
    >
      <button
        type="button"
        :aria-pressed="selected.length === 0"
        :disabled="disabled"
        :class="[chipBase, selected.length === 0 ? chipOn : chipOff]"
        data-testid="category-chip-all"
        @click="emit('update:selected', [])"
      >
        All
      </button>
      <button
        v-for="c in categories"
        :key="c.slug"
        type="button"
        :aria-pressed="selected.includes(c.slug)"
        :disabled="disabled"
        :class="[chipBase, selected.includes(c.slug) ? chipOn : chipOff]"
        :data-testid="`category-chip-${c.slug}`"
        @click="toggle(c.slug)"
      >
        <span>{{ c.name }}</span> <span class="text-xs opacity-75">{{ c.item_count }}<span class="sr-only"> items</span></span>
      </button>
    </div>

    <button
      type="button"
      role="switch"
      :aria-checked="showEmpty"
      :disabled="disabled"
      class="group inline-flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300"
      data-testid="show-empty-toggle"
      @click="emit('update:showEmpty', !showEmpty)"
    >
      <span
        class="relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition group-focus-visible:ring-2 group-focus-visible:ring-slate-900 dark:group-focus-visible:ring-slate-100"
        :class="showEmpty ? 'bg-slate-900 dark:bg-slate-100' : 'bg-slate-300 dark:bg-slate-700'"
        aria-hidden="true"
      >
        <span
          class="inline-block h-4 w-4 rounded-full bg-white shadow transition dark:bg-slate-900"
          :class="showEmpty ? 'translate-x-4' : 'translate-x-0.5'"
        />
      </span>
      Show empty categories
    </button>
  </div>
</template>
