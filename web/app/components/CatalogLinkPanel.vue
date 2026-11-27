<script setup lang="ts">
import { ref } from "vue";
import { catalogEntryPath, matchLabel } from "~/utils/catalog";
import type { CatalogSuggestion, Item } from "~/types/api";

/**
 * Link one of your items to the global catalog, or unlink it (ADR 0004).
 * For a free-form item, "Find in catalog" asks the API for likely matches;
 * nothing is linked until you pick one. Linking only sets the link: your
 * name and details stay as you wrote them.
 */
const props = defineProps<{
  item: Item;
  categorySlug: string;
  /** Human label of the category's identifier attribute ("Set number"), if any. */
  identifierTitle?: string | null;
}>();

const emit = defineEmits<{ (e: "changed", item: Item): void }>();

const { suggestions: fetchSuggestions, link, unlink } = useCatalogActions();

const open = ref(false);
const loading = ref(false);
const busy = ref(false);
const error = ref<string | null>(null);
const suggestions = ref<CatalogSuggestion[]>([]);

async function onFind() {
  open.value = true;
  loading.value = true;
  error.value = null;
  try {
    const res = await fetchSuggestions(props.item.collection_id, props.item.id);
    suggestions.value = res.suggestions;
  } catch (e) {
    console.error(e);
    error.value = "Couldn’t search the catalog. Please try again.";
  } finally {
    loading.value = false;
  }
}

async function onLink(entry: CatalogSuggestion) {
  busy.value = true;
  error.value = null;
  try {
    const updated = await link(props.item.collection_id, props.item.id, entry.id);
    open.value = false;
    suggestions.value = [];
    emit("changed", updated);
  } catch (e) {
    console.error(e);
    error.value = "Couldn’t link it. Please try again.";
  } finally {
    busy.value = false;
  }
}

async function onUnlink() {
  busy.value = true;
  error.value = null;
  try {
    emit("changed", await unlink(props.item.collection_id, props.item.id));
  } catch (e) {
    console.error(e);
    error.value = "Couldn’t unlink it. Please try again.";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="mt-3 border-t border-slate-100 pt-3 dark:border-slate-800" :data-testid="`catalog-link-${item.id}`">
    <div v-if="item.catalog_entry_id" class="flex flex-wrap items-center justify-between gap-2 text-xs">
      <NuxtLink
        :to="catalogEntryPath(categorySlug, item.catalog_entry_id)"
        class="inline-flex items-center gap-1 font-medium text-emerald-800 hover:underline dark:text-emerald-200"
        data-testid="catalog-linked"
      >
        <Icon name="lucide:book-open-check" class="h-3.5 w-3.5" aria-hidden="true" />
        Linked to the catalog
      </NuxtLink>
      <button
        type="button"
        :disabled="busy"
        class="rounded-md border border-slate-200 px-2 py-1 text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
        @click="onUnlink"
      >
        Unlink
      </button>
    </div>

    <template v-else>
      <button
        v-if="!open"
        type="button"
        class="inline-flex items-center gap-1 text-xs font-medium text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-slate-50"
        @click="onFind"
      >
        <Icon name="lucide:search" class="h-3.5 w-3.5" aria-hidden="true" />
        Find in catalog
      </button>

      <div v-else class="space-y-2" data-testid="catalog-suggestions">
        <div class="flex items-center justify-between">
          <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">Possible catalog matches</p>
          <button
            type="button"
            class="text-xs text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
            @click="open = false"
          >
            Close
          </button>
        </div>
        <p v-if="loading" class="text-xs text-slate-500 dark:text-slate-400">Searching the catalog…</p>
        <ul v-else-if="suggestions.length > 0" class="space-y-1">
          <li
            v-for="s in suggestions"
            :key="s.id"
            class="flex items-center justify-between gap-2 rounded-md bg-slate-50 px-2 py-1.5 dark:bg-slate-800"
            data-testid="catalog-suggestion"
          >
            <div class="min-w-0 text-xs">
              <p class="truncate font-medium text-slate-900 dark:text-slate-50">{{ s.name }}</p>
              <p class="text-slate-500 dark:text-slate-400">
                <span v-if="identifierTitle && s.identifier">{{ identifierTitle }} {{ s.identifier }} · </span>{{ matchLabel(s.match, identifierTitle ?? null) }}
              </p>
            </div>
            <button
              type="button"
              :disabled="busy"
              class="shrink-0 rounded-md bg-slate-900 px-2 py-1 text-xs font-medium text-white hover:bg-slate-800 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
              :aria-label="`Link to ${s.name}`"
              @click="onLink(s)"
            >
              Link
            </button>
          </li>
        </ul>
        <p v-else-if="!error" class="text-xs text-slate-500 dark:text-slate-400" data-testid="catalog-suggestions-empty">
          No likely matches.
          <NuxtLink :to="`/categories/${categorySlug}`" class="underline">Browse the catalog</NuxtLink>
        </p>
      </div>
    </template>

    <p v-if="error" role="alert" class="mt-1 text-xs text-rose-700 dark:text-rose-300">{{ error }}</p>
  </div>
</template>
