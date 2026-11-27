<script setup lang="ts">
import { computed, ref, useId } from "vue";
import { useDebouncedRef } from "~/composables/useDebouncedRef";
import { SEARCH_DEBOUNCE_MS } from "~/utils/search";
import { collapseWhitespace } from "~/utils/text";

/**
 * Search box + results for the signed-in user's own items. The parent owns
 * the raw text (v-model) so it can keep it in the URL; this component
 * normalizes and debounces it before searching. Renders no results area
 * while the box is blank, so the parent can show its normal content then.
 */
const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ (e: "update:modelValue", value: string): void }>();

const inputId = useId();
const input = ref<HTMLInputElement | null>(null);

const text = computed({
  get: () => props.modelValue,
  set: (value: string) => emit("update:modelValue", value),
});
const normalized = computed(() => collapseWhitespace(props.modelValue));
const term = useDebouncedRef(normalized, SEARCH_DEBOUNCE_MS);
const { data, pending, error, refresh } = useMyItemSearch(term);

const active = computed(() => normalized.value !== "");
const searching = computed(() => active.value && (normalized.value !== term.value || pending.value));
const results = computed(() => data.value?.items ?? []);

const status = computed(() => {
  // Announce only settled results: "Searching…" on every debounced
  // keystroke would be noise for screen-reader users (it's shown visually).
  if (!active.value || error.value || searching.value) return "";
  const n = results.value.length;
  if (n === 0) return "No items found";
  return `${n}${data.value?.has_more ? "+" : ""} item${n === 1 ? "" : "s"} found`;
});

// On mobile the keyboard's Search key submits the form: dismiss the keyboard
// so the results are visible.
function onSubmit() {
  input.value?.blur();
}
</script>

<template>
  <div class="space-y-3">
    <form role="search" class="relative" @submit.prevent="onSubmit">
      <label :for="inputId" class="sr-only">Search your items</label>
      <Icon
        name="lucide:search"
        class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400 dark:text-slate-500"
        aria-hidden="true"
      />
      <input
        :id="inputId"
        ref="input"
        v-model="text"
        type="search"
        inputmode="search"
        enterkeyhint="search"
        autocomplete="off"
        placeholder="Search your items…"
        class="block w-full rounded-lg border-slate-300 bg-white py-2 pl-9 pr-3 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        data-testid="item-search-input"
      >
    </form>

    <p class="sr-only" role="status" aria-live="polite" data-testid="item-search-status">{{ status }}</p>

    <template v-if="active">
      <div
        v-if="error"
        role="alert"
        class="flex items-center justify-between gap-3 rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
        data-testid="item-search-error"
      >
        <span>Couldn’t search your items. Please try again.</span>
        <button
          type="button"
          class="shrink-0 rounded-md border border-rose-200 px-2 py-1 text-xs font-medium hover:bg-rose-100 dark:border-rose-800 dark:hover:bg-rose-900"
          @click="refresh()"
        >
          Try again
        </button>
      </div>

      <div
        v-else-if="searching && results.length === 0"
        class="text-sm text-slate-500 dark:text-slate-400"
        aria-hidden="true"
        data-testid="item-search-searching"
      >
        Searching…
      </div>

      <ul
        v-else-if="results.length > 0"
        class="space-y-2 transition-opacity"
        :class="{ 'opacity-60': searching }"
        :aria-busy="searching"
        data-testid="item-search-results"
      >
        <li v-for="r in results" :key="r.id" data-testid="item-search-result">
          <NuxtLink
            :to="`/my/${r.collection_id}`"
            class="block rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 hover:ring-slate-300 dark:bg-slate-900 dark:ring-slate-800 dark:hover:ring-slate-700"
          >
            <div class="flex items-baseline justify-between gap-3">
              <h3 class="truncate text-base font-semibold text-slate-900 dark:text-slate-50">{{ r.name }}</h3>
              <span v-if="r.quantity > 1" class="shrink-0 text-xs text-slate-500 dark:text-slate-400">
                ×{{ r.quantity }}
              </span>
            </div>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">
              {{ r.collection_name }} · {{ r.category_name }}
            </p>
            <p v-if="r.matched_field" class="mt-1 truncate text-xs text-slate-600 dark:text-slate-300">
              {{ r.matched_field.label }}: {{ r.matched_field.value }}
            </p>
          </NuxtLink>
        </li>
      </ul>

      <div
        v-else
        class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
        data-testid="item-search-empty"
      >
        Nothing in your collections matches “{{ term }}”.
      </div>

      <p
        v-if="!error && data?.has_more"
        class="text-center text-xs text-slate-500 dark:text-slate-400"
        data-testid="item-search-more"
      >
        Showing the first {{ results.length }} matches. Try a more specific search to narrow them down.
      </p>
    </template>
  </div>
</template>
