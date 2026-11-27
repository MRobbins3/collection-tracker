<script setup lang="ts">
import { computed } from "vue";
import type { Collection, OverviewCategory } from "~/types/api";

/**
 * The overview's "Collections" view: your collections grouped under their
 * category. A category you added with no collections yet offers "Start a
 * collection" and "Remove". Categories come pre-filtered and pre-sorted.
 */
const props = defineProps<{
  categories: OverviewCategory[];
  collections: Collection[];
  busySlug: string | null;
}>();
const emit = defineEmits<{
  deleteCollection: [id: string, name: string];
  startCollection: [slug: string, name: string];
  unpin: [slug: string];
}>();

const bySlug = computed(() => {
  const m = new Map<string, Collection[]>();
  for (const c of props.collections) {
    const list = m.get(c.category_slug) ?? [];
    list.push(c);
    m.set(c.category_slug, list);
  }
  return m;
});

function itemsText(n: number): string {
  return `${n} item${n === 1 ? "" : "s"}`;
}
</script>

<template>
  <div class="space-y-5" data-testid="my-collections-list">
    <section
      v-for="cat in categories"
      :key="cat.slug"
      :aria-labelledby="`category-group-heading-${cat.slug}`"
      class="space-y-2"
      :data-testid="`category-group-${cat.slug}`"
    >
      <div class="flex items-baseline justify-between gap-3">
        <h2
          :id="`category-group-heading-${cat.slug}`"
          class="text-sm font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-300"
        >
          {{ cat.name }}
        </h2>
        <span class="text-xs text-slate-500 dark:text-slate-400">
          {{ cat.item_count > 0 ? itemsText(cat.item_count) : "Nothing added yet" }}
        </span>
      </div>

      <ul v-if="(bySlug.get(cat.slug) ?? []).length > 0" class="space-y-2">
        <li
          v-for="c in bySlug.get(cat.slug)"
          :key="c.id"
          class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
          :data-testid="`my-collection-${c.id}`"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <NuxtLink :to="`/my/${c.id}`" class="block">
                <h3 class="truncate text-base font-semibold text-slate-900 dark:text-slate-50">
                  {{ c.name }}
                </h3>
                <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
                  {{ itemsText(c.item_count) }}
                </p>
              </NuxtLink>
            </div>
            <button
              type="button"
              class="shrink-0 rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              :data-testid="`delete-collection-${c.id}`"
              @click="emit('deleteCollection', c.id, c.name)"
            >
              Delete
            </button>
          </div>
        </li>
      </ul>

      <div
        v-else
        class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-dashed border-slate-300 p-4 dark:border-slate-700"
      >
        <p class="text-sm text-slate-600 dark:text-slate-400">No collections here yet.</p>
        <div class="flex gap-2">
          <button
            type="button"
            :disabled="busySlug === cat.slug"
            class="inline-flex h-9 items-center rounded-lg bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
            :data-testid="`start-collection-${cat.slug}`"
            @click="emit('startCollection', cat.slug, cat.name)"
          >
            Start a collection
          </button>
          <button
            v-if="cat.pinned"
            type="button"
            :disabled="busySlug === cat.slug"
            :aria-label="`Remove ${cat.name} from my collection`"
            class="inline-flex h-9 items-center rounded-lg border border-slate-300 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
            :data-testid="`remove-category-${cat.slug}`"
            @click="emit('unpin', cat.slug)"
          >
            Remove
          </button>
        </div>
      </div>
    </section>
  </div>
</template>
