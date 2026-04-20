<script setup lang="ts">
import { ref, watch } from "vue";
import ItemForm from "./ItemForm.vue";
import type { AttributeSchema, CatalogEntry } from "~/types/api";

const props = defineProps<{
  categorySlug: string;
  schema?: AttributeSchema;
}>();

const emit = defineEmits<{
  (e: "create", payload: { name: string; quantity: number; condition?: string; attributes: Record<string, unknown> }): void;
}>();

const { search } = useCatalogSearch();

const query = ref("");
const results = ref<CatalogEntry[]>([]);
const showingForm = ref(false);
const prefilledName = ref("");
const prefilledAttributes = ref<Record<string, unknown>>({});

let searchTimer: ReturnType<typeof setTimeout> | null = null;

watch(query, (q) => {
  if (searchTimer) clearTimeout(searchTimer);
  // Debounce 200ms to avoid hammering the catalog on every keystroke.
  searchTimer = setTimeout(async () => {
    if (!q.trim()) {
      results.value = [];
      return;
    }
    try {
      const res = await search(props.categorySlug, q);
      results.value = res.entries;
    } catch {
      results.value = [];
    }
  }, 200);
});

function openManualForm(fromQuery: boolean) {
  prefilledName.value = fromQuery ? query.value.trim() : "";
  prefilledAttributes.value = {};
  showingForm.value = true;
}

function pickFromCatalog(entry: CatalogEntry) {
  prefilledName.value = entry.name;
  prefilledAttributes.value = { ...(entry.attributes ?? {}) };
  showingForm.value = true;
}

function onCancel() {
  showingForm.value = false;
  query.value = "";
  results.value = [];
}
</script>

<template>
  <section
    class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200"
    data-testid="new-item-panel"
  >
    <h2 class="text-sm font-semibold text-slate-900">Add an item</h2>

    <div v-if="!showingForm" class="mt-3 space-y-2">
      <label class="block">
        <span class="sr-only">Search the catalog</span>
        <input
          v-model="query"
          type="search"
          inputmode="search"
          placeholder="Search — or type to add manually"
          class="block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900"
          data-testid="new-item-search"
        />
      </label>

      <ul v-if="results.length > 0" class="space-y-1" data-testid="new-item-results">
        <li v-for="entry in results" :key="entry.id">
          <button
            type="button"
            class="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-left text-sm hover:bg-slate-50"
            @click="pickFromCatalog(entry)"
          >
            <div class="font-medium text-slate-900">{{ entry.name }}</div>
            <div v-if="entry.description" class="text-xs text-slate-500">{{ entry.description }}</div>
          </button>
        </li>
      </ul>

      <p
        v-if="query && results.length === 0"
        class="text-xs text-slate-500"
        data-testid="new-item-empty-catalog"
      >
        Not in our list yet — you can enter it manually.
      </p>

      <button
        type="button"
        class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800"
        data-testid="new-item-manual"
        @click="openManualForm(query.trim().length > 0)"
      >
        {{ query.trim() ? `Add “${query.trim()}” manually` : "Enter item manually" }}
      </button>
    </div>

    <ItemForm
      v-else
      class="mt-3"
      :schema="schema"
      :initial="{ name: prefilledName, attributes: prefilledAttributes, quantity: 1 }"
      submit-label="Add to collection"
      @submit="
        (payload) => {
          emit('create', payload);
          showingForm = false;
          query = '';
          results = [];
        }
      "
      @cancel="onCancel"
    />
  </section>
</template>
