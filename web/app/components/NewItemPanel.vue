<script setup lang="ts">
import { ref, watch } from "vue";
import ItemForm from "./ItemForm.vue";
import type { AttributeSchema, CatalogEntry } from "~/types/api";
import { collapseWhitespace } from "~/utils/text";

/**
 * Add an item: type to search the category's catalog, pick a match to link
 * the new item to it with name and details filled in (still editable,
 * ADR 0004), or enter it manually. Manual entry is always available, and
 * can also suggest the item for the catalog (ADR 0012): the parent creates
 * the item, then sends the suggestion with it, so the item is linked when
 * an admin approves.
 */
const props = defineProps<{
  categorySlug: string;
  schema?: AttributeSchema;
  /** Human label of the category's identifier attribute ("Set number"), if any. */
  identifierTitle?: string | null;
  /** The category isn't on the public list yet (ADR 0013), so it can't take catalog suggestions. */
  noSuggestions?: boolean;
}>();

const emit = defineEmits<{
  (
    e: "create",
    payload: {
      name: string;
      quantity: number;
      condition?: string;
      attributes: Record<string, unknown>;
      catalog_entry_id?: string;
      /** Also suggest this item for the catalog. */
      suggest?: boolean;
    },
  ): void;
}>();

const { search } = useCatalogSearch();

const query = ref("");
const results = ref<CatalogEntry[]>([]);
const searched = ref("");
const showingForm = ref(false);
const prefilledName = ref("");
const prefilledAttributes = ref<Record<string, unknown>>({});
const linkedEntry = ref<CatalogEntry | null>(null);
const suggestToCatalog = ref(false);

let searchTimer: ReturnType<typeof setTimeout> | null = null;
// Bumped by every new search: a slower, older response must not overwrite
// the results for what's in the box now.
let generation = 0;

watch(query, (raw) => {
  if (searchTimer) clearTimeout(searchTimer);
  const q = collapseWhitespace(raw);
  const gen = ++generation;
  if (!q) {
    results.value = [];
    searched.value = "";
    return;
  }
  searchTimer = setTimeout(async () => {
    try {
      const res = await search(props.categorySlug, q);
      if (gen !== generation) return;
      results.value = res.entries;
    } catch {
      if (gen !== generation) return;
      results.value = [];
    }
    searched.value = q;
  }, 200);
});

function openManualForm(fromQuery: boolean, suggest = false) {
  prefilledName.value = fromQuery ? collapseWhitespace(query.value) : "";
  prefilledAttributes.value = {};
  linkedEntry.value = null;
  suggestToCatalog.value = suggest;
  showingForm.value = true;
}

function pickFromCatalog(entry: CatalogEntry) {
  prefilledName.value = entry.name;
  prefilledAttributes.value = { ...(entry.attributes ?? {}) };
  linkedEntry.value = entry;
  showingForm.value = true;
}

function reset() {
  showingForm.value = false;
  linkedEntry.value = null;
  suggestToCatalog.value = false;
  query.value = "";
  results.value = [];
}

function onSubmit(payload: { name: string; quantity: number; condition?: string; attributes: Record<string, unknown> }) {
  if (linkedEntry.value) emit("create", { ...payload, catalog_entry_id: linkedEntry.value.id });
  else emit("create", suggestToCatalog.value ? { ...payload, suggest: true } : payload);
  reset();
}
</script>

<template>
  <section
    class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="new-item-panel"
  >
    <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Add an item</h2>

    <div v-if="!showingForm" class="mt-3 space-y-2">
      <label class="block">
        <span class="sr-only">Search the catalog</span>
        <input
          v-model="query"
          type="search"
          inputmode="search"
          autocomplete="off"
          placeholder="Search the catalog — or type to add manually"
          class="block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
          data-testid="new-item-search"
        >
      </label>

      <ul v-if="results.length > 0" class="space-y-1" aria-label="Catalog matches" data-testid="new-item-results">
        <li v-for="entry in results" :key="entry.id">
          <button
            type="button"
            class="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-left text-sm hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:hover:bg-slate-800"
            data-testid="new-item-result"
            @click="pickFromCatalog(entry)"
          >
            <div class="font-medium text-slate-900 dark:text-slate-100">{{ entry.name }}</div>
            <div
              v-if="(identifierTitle && entry.identifier) || entry.description"
              class="text-xs text-slate-500 dark:text-slate-400"
            >
              <span v-if="identifierTitle && entry.identifier">{{ identifierTitle }} {{ entry.identifier }}</span>
              <span v-if="identifierTitle && entry.identifier && entry.description"> · </span>
              <span v-if="entry.description">{{ entry.description }}</span>
            </div>
          </button>
        </li>
      </ul>

      <p
        v-if="searched && results.length === 0"
        class="text-xs text-slate-500 dark:text-slate-400"
        data-testid="new-item-empty-catalog"
      >
        Not in the catalog yet — you can enter it manually.
        <button
          v-if="!noSuggestions"
          type="button"
          class="font-medium text-slate-900 underline underline-offset-2 dark:text-slate-100"
        data-testid="new-item-suggest-open"
          @click="openManualForm(true, true)"
        >
          Can’t find it? Suggest it for the catalog
        </button>
      </p>

      <button
        type="button"
        class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        data-testid="new-item-manual"
        @click="openManualForm(query.trim().length > 0)"
      >
        {{ query.trim() ? `Add “${query.trim()}” manually` : "Enter item manually" }}
      </button>
    </div>

    <template v-else>
      <div
        v-if="linkedEntry"
        class="mt-3 flex items-start justify-between gap-3 rounded-md bg-emerald-50 p-3 text-xs text-emerald-900 ring-1 ring-emerald-200 dark:bg-emerald-950 dark:text-emerald-100 dark:ring-emerald-800"
        data-testid="new-item-linked"
      >
        <p>
          Linked to <span class="font-semibold">{{ linkedEntry.name }}</span> in the catalog. You can still change
          the name and details.
        </p>
        <button
          type="button"
          class="shrink-0 underline hover:no-underline"
          data-testid="new-item-unlink"
          @click="linkedEntry = null"
        >
          Don’t link
        </button>
      </div>
      <label
        v-if="!linkedEntry && !noSuggestions"
        class="mt-3 flex items-start gap-2 text-xs text-slate-700 dark:text-slate-300"
      >
        <input
          v-model="suggestToCatalog"
          type="checkbox"
          class="mt-0.5 rounded border-slate-300 dark:border-slate-600 dark:bg-slate-900"
          data-testid="new-item-suggest"
        >
        <span>
          Also suggest it for the catalog. An admin reviews it first; your item is linked once it’s approved.
        </span>
      </label>
      <ItemForm
        class="mt-3"
        :schema="schema"
        :initial="{ name: prefilledName, attributes: prefilledAttributes, quantity: 1 }"
        submit-label="Add to collection"
        @submit="onSubmit"
        @cancel="reset"
      />
    </template>
  </section>
</template>
