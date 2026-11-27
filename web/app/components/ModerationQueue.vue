<script setup lang="ts">
import { onMounted, ref, useId, watch } from "vue";
import { kindLabel } from "~/utils/moderation";
import type { ModerationKind, QueueItem } from "~/types/api";

/**
 * The admin moderation queue (ADR 0012): pending suggestions and
 * corrections, oldest first, filterable by kind and category, with
 * "Show more". A filter change starts over; a generation counter drops any
 * response that arrives for an older filter.
 */
defineProps<{ categories: { slug: string; name: string }[] }>();

const { queue } = useModerationAdmin();

const ids = { kind: useId(), category: useId() };
const kind = ref<ModerationKind | "">("");
const categorySlug = ref("");
const requests = ref<QueueItem[]>([]);
const total = ref(0);
const hasMore = ref(false);
const loading = ref(false);
const error = ref(false);
let generation = 0;

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });

async function load(reset: boolean) {
  const gen = reset ? ++generation : generation;
  loading.value = true;
  error.value = false;
  try {
    const page = await queue({
      kind: kind.value,
      categorySlug: categorySlug.value,
      offset: reset ? 0 : requests.value.length,
    });
    if (gen !== generation) return;
    requests.value = reset ? page.requests : [...requests.value, ...page.requests];
    total.value = page.total;
    hasMore.value = page.has_more;
  } catch (e) {
    if (gen !== generation) return;
    console.error(e);
    error.value = true;
  } finally {
    if (gen === generation) loading.value = false;
  }
}

watch([kind, categorySlug], () => load(true));
onMounted(() => load(true));

const selectClass =
  "mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-sm shadow-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100";

</script>

<template>
  <div class="space-y-4" data-testid="moderation-queue">
    <div class="grid grid-cols-2 gap-3">
      <div>
        <label :for="ids.kind" class="block text-xs font-medium text-slate-700 dark:text-slate-300">Type</label>
        <select :id="ids.kind" v-model="kind" :class="selectClass" data-testid="queue-kind">
          <option value="">Everything</option>
          <option value="catalog_entry">New entries</option>
          <option value="catalog_correction">Corrections</option>
          <option value="category">New categories</option>
        </select>
      </div>
      <div>
        <label :for="ids.category" class="block text-xs font-medium text-slate-700 dark:text-slate-300">Category</label>
        <select :id="ids.category" v-model="categorySlug" :class="selectClass" data-testid="queue-category">
          <option value="">All categories</option>
          <option v-for="c in categories" :key="c.slug" :value="c.slug">{{ c.name }}</option>
        </select>
      </div>
    </div>

    <p class="text-sm text-slate-600 dark:text-slate-400" data-testid="queue-status" aria-live="polite">
      <template v-if="loading && requests.length === 0">Loading…</template>
      <template v-else>{{ total }} waiting for review</template>
    </p>

    <div
      v-if="error"
      role="alert"
      class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
    >
      We couldn’t load the queue.
      <button type="button" class="underline" @click="load(requests.length === 0)">Try again</button>
    </div>

    <div
      v-if="!loading && !error && requests.length === 0"
      class="rounded-xl bg-slate-100 p-4 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      data-testid="queue-empty"
    >
      Nothing waiting. Nice work.
    </div>

    <ul v-if="requests.length > 0" class="space-y-2">
      <li v-for="r in requests" :key="r.id" data-testid="queue-item">
        <NuxtLink
          :to="`/admin/moderation/${r.id}`"
          class="block rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 hover:ring-slate-400 dark:bg-slate-900 dark:ring-slate-800 dark:hover:ring-slate-600"
        >
          <p class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
            {{ kindLabel(r.kind) }} · {{ r.category_name }}
          </p>
          <p class="font-medium text-slate-900 dark:text-slate-50">
            {{ r.name }}<span v-if="r.identifier" class="font-normal text-slate-500 dark:text-slate-400"> · {{ r.identifier }}</span>
          </p>
          <p v-if="r.note" class="mt-1 text-sm text-slate-600 dark:text-slate-300">“{{ r.note }}”</p>
          <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
            From {{ r.submitter.display_name }} · {{ dateFormat.format(new Date(r.created_at)) }}
          </p>
        </NuxtLink>
      </li>
    </ul>

    <button
      v-if="hasMore"
      type="button"
      :disabled="loading"
      class="inline-flex h-10 w-full items-center justify-center rounded-md border border-slate-300 bg-white text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
      data-testid="queue-more"
      @click="load(false)"
    >
      {{ loading ? "Loading…" : "Show more" }}
    </button>
  </div>
</template>
