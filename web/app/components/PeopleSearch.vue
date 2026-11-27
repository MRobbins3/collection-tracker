<script setup lang="ts">
import { computed, ref, useId, watch } from "vue";
import PersonRow from "./PersonRow.vue";
import { useDebouncedRef } from "~/composables/useDebouncedRef";
import type { PersonCard } from "~/types/api";
import { SEARCH_DEBOUNCE_MS } from "~/utils/search";
import { collapseWhitespace } from "~/utils/text";
import { socialErrorMessage } from "~/utils/social";

/**
 * Find people by handle or name. The API only returns profiles you're
 * allowed to see (a Private profile you have no tie to never appears).
 * Results link to profiles, where the friend and follow buttons live.
 */
const MIN_LENGTH = 2;

const { searchPeople } = useSocialActions();
const inputId = useId();
const text = ref("");
const normalized = computed(() => collapseWhitespace(text.value).replace(/^@/, ""));
const term = useDebouncedRef(normalized, SEARCH_DEBOUNCE_MS);

const results = ref<PersonCard[]>([]);
const loading = ref(false);
const error = ref("");
// Drops stale answers: only the latest search may write results.
let generation = 0;

watch(term, async (q) => {
  const mine = ++generation;
  error.value = "";
  if (q.length < MIN_LENGTH) {
    results.value = [];
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const found = await searchPeople(q);
    if (mine === generation) results.value = found;
  } catch (e) {
    if (mine === generation) error.value = socialErrorMessage(e);
  } finally {
    if (mine === generation) loading.value = false;
  }
});

const active = computed(() => normalized.value.length >= MIN_LENGTH);
const settled = computed(() => active.value && term.value === normalized.value && !loading.value);
const status = computed(() => {
  if (!settled.value || error.value) return "";
  const n = results.value.length;
  return n === 0 ? "No one found" : `${n} ${n === 1 ? "person" : "people"} found`;
});
</script>

<template>
  <div class="space-y-3">
    <form role="search" class="relative" @submit.prevent>
      <label :for="inputId" class="sr-only">Find people by name or handle</label>
      <Icon
        name="lucide:search"
        class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400 dark:text-slate-500"
        aria-hidden="true"
      />
      <input
        :id="inputId"
        v-model="text"
        type="search"
        inputmode="search"
        enterkeyhint="search"
        autocomplete="off"
        placeholder="Find people by name or @handle…"
        class="block w-full rounded-lg border-slate-300 bg-white py-2 pl-9 pr-3 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        data-testid="people-search-input"
      >
    </form>
    <p class="sr-only" role="status" aria-live="polite" data-testid="people-search-status">{{ status }}</p>

    <template v-if="active">
      <p v-if="error" role="alert" class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100">
        {{ error }}
      </p>
      <p v-else-if="!settled" class="text-sm text-slate-500 dark:text-slate-400">Searching…</p>
      <p
        v-else-if="results.length === 0"
        class="rounded-xl bg-slate-100 p-4 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
        data-testid="people-search-empty"
      >
        No one matches “{{ term }}”.
      </p>
      <ul v-else class="space-y-2" data-testid="people-search-results">
        <PersonRow v-for="p in results" :key="p.handle" :person="p" />
      </ul>
    </template>
  </div>
</template>
