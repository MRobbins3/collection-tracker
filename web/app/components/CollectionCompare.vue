<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from "vue";
import AddToCollection from "./AddToCollection.vue";
import UserAvatar from "./UserAvatar.vue";
import type { CompareBucket, CompareResponse, CompareRow } from "~/types/api";
import {
  COMPARE_BUCKETS,
  bucketLabel,
  defaultCategory,
  emptyBucketMessage,
  firstName,
  sideSummary,
} from "~/utils/compare";

/**
 * Your collection next to someone else's, in one category: what you both
 * have, what only you have, what only they have, and catalog items neither
 * of you has. The parent keeps the category and tab in the URL
 * (v-model:category, v-model:bucket). Loads in the browser (signed-in only).
 */
const props = defineProps<{ handle: string; category?: string; bucket: CompareBucket }>();
const emit = defineEmits<{
  (e: "update:category", slug: string | undefined): void;
  (e: "update:bucket", bucket: CompareBucket): void;
}>();

const PAGE = 50;
const { fetchCompare } = useCompareActions();
const baseId = useId();

const res = ref<CompareResponse | null>(null);
const rows = ref<CompareRow[]>([]);
const hasMore = ref(false);
const state = ref<"loading" | "ready" | "hidden" | "self" | "error">("loading");
const loadingMore = ref(false);
const moreError = ref(false);
const notice = ref("");
const openAdd = ref<string | null>(null);
// A reload with the previous results still on screen (new tab or category).
const refreshing = ref(false);
// Bumped by every fresh load: answers for an older one are dropped.
let generation = 0;

const other = computed(() => res.value?.other);
const otherName = computed(() => other.value?.display_name ?? "");
const counts = computed(() => res.value?.counts ?? null);
const categoryOptions = computed(() => {
  const list = res.value?.categories ?? [];
  const cur = res.value?.category;
  // A category neither of you has items in yet (e.g. picked from a link).
  if (cur && !list.some((c) => c.slug === cur.slug)) {
    return [...list, { slug: cur.slug, name: cur.name, public: cur.public, mine_items: 0, their_items: 0 }];
  }
  return list;
});

async function load() {
  const mine = ++generation;
  openAdd.value = null;
  moreError.value = false;
  loadingMore.value = false;
  if (!res.value) state.value = "loading";
  refreshing.value = true;
  try {
    const out = await fetchCompare(props.handle, { category: props.category, bucket: props.bucket, limit: PAGE });
    if (mine !== generation) return;
    if (out === "hidden") {
      state.value = "hidden";
      return;
    }
    if (out === "unknown_category") {
      notice.value = "That category isn’t available to compare, so we picked another.";
      emit("update:category", undefined);
      return;
    }
    res.value = out;
    rows.value = out.items;
    hasMore.value = out.has_more;
    state.value = "ready";
    if (!props.category) {
      const slug = defaultCategory(out.categories);
      if (slug) emit("update:category", slug);
    }
  } catch (e) {
    if (mine !== generation) return;
    const status = (e as { statusCode?: number })?.statusCode;
    state.value = status === 400 && !res.value ? "self" : "error";
  } finally {
    if (mine === generation) refreshing.value = false;
  }
}

async function loadMore() {
  const mine = generation;
  loadingMore.value = true;
  moreError.value = false;
  try {
    const out = await fetchCompare(props.handle, {
      category: props.category,
      bucket: props.bucket,
      limit: PAGE,
      offset: rows.value.length,
    });
    if (mine !== generation) return;
    if (typeof out === "string") {
      moreError.value = true;
      return;
    }
    rows.value = [...rows.value, ...out.items];
    hasMore.value = out.has_more;
  } catch {
    if (mine === generation) moreError.value = true;
  } finally {
    if (mine === generation) loadingMore.value = false;
  }
}

onMounted(load);
watch(
  () => [props.handle, props.category, props.bucket],
  () => void load(),
);

function selectCategory(e: Event) {
  notice.value = "";
  emit("update:category", (e.target as HTMLSelectElement).value || undefined);
}

function selectBucket(b: CompareBucket) {
  if (b !== props.bucket) emit("update:bucket", b);
}

// Arrow keys move between tabs (WAI-ARIA tabs pattern).
function onTabKey(e: KeyboardEvent, index: number) {
  const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
  if (!step) return;
  e.preventDefault();
  const next = COMPARE_BUCKETS[(index + step + COMPARE_BUCKETS.length) % COMPARE_BUCKETS.length]!;
  selectBucket(next);
  document.getElementById(`${baseId}-tab-${next}`)?.focus();
}

function onAdded(payload: { entryName: string; collectionName: string }) {
  notice.value = `Added ${payload.entryName} to ${payload.collectionName}.`;
  void load();
}

function catalogLink(row: CompareRow): string | null {
  const slug = res.value?.category?.slug;
  return row.catalog_entry_id && slug ? `/categories/${slug}/catalog/${row.catalog_entry_id}` : null;
}

const emptyMessage = computed(() =>
  res.value?.category
    ? emptyBucketMessage(props.bucket, {
        otherName: otherName.value,
        categoryName: res.value.category.name,
        catalogEntries: res.value.category.catalog_entries,
      })
    : "",
);

const card = "rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800";
const empty = "rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300";
const link = "font-medium text-slate-700 underline underline-offset-2 hover:text-slate-900 dark:text-slate-200 dark:hover:text-white";
const secondary =
  "inline-flex h-9 items-center rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800";
</script>

<template>
  <section class="space-y-4" data-testid="collection-compare">
    <p v-if="state === 'loading'" class="text-sm text-slate-500 dark:text-slate-400" data-testid="compare-loading">
      Loading comparison…
    </p>

    <section v-else-if="state === 'hidden'" :class="empty" class="space-y-3" data-testid="compare-hidden">
      <h1 class="text-lg font-semibold text-slate-900 dark:text-slate-50">You can’t compare with this person</h1>
      <p>They don’t share their collections with you, or there’s no profile here.</p>
      <NuxtLink :to="`/u/${handle}`" :class="link">Back to their profile</NuxtLink>
    </section>

    <section v-else-if="state === 'self'" :class="empty" class="space-y-3" data-testid="compare-self">
      <h1 class="text-lg font-semibold text-slate-900 dark:text-slate-50">That’s you</h1>
      <p>Open a friend’s profile and choose Compare collections.</p>
      <NuxtLink to="/people" :class="link">Go to People</NuxtLink>
    </section>

    <div
      v-else-if="state === 'error' && !res"
      role="alert"
      class="rounded-xl bg-rose-50 p-4 text-sm text-rose-800 dark:bg-rose-950 dark:text-rose-200"
    >
      We couldn’t load the comparison.
      <button type="button" class="font-medium underline" @click="load">Try again</button>
    </div>

    <template v-else-if="res && other">
      <header class="flex items-center gap-3">
        <UserAvatar :name="other.display_name" :color="other.avatar_color" />
        <div class="min-w-0">
          <h1 class="truncate text-xl font-bold tracking-tight text-slate-900 dark:text-slate-50">
            Compare with {{ other.display_name }}
          </h1>
          <NuxtLink :to="`/u/${other.handle}`" class="text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100">
            @{{ other.handle }}
          </NuxtLink>
        </div>
      </header>

      <p v-if="notice" role="status" class="rounded-lg bg-emerald-50 p-3 text-sm text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100" data-testid="compare-notice">
        {{ notice }}
      </p>
      <div
        v-if="state === 'error'"
        role="alert"
        class="rounded-xl bg-rose-50 p-4 text-sm text-rose-800 dark:bg-rose-950 dark:text-rose-200"
      >
        We couldn’t load the comparison.
        <button type="button" class="font-medium underline" @click="load">Try again</button>
      </div>

      <p v-if="categoryOptions.length === 0" :class="empty" data-testid="compare-no-categories">
        Neither of you has anything in a collection yet. Add some items, then come back to compare.
      </p>

      <template v-else>
        <label class="block">
          <span class="block text-sm font-medium text-slate-700 dark:text-slate-300">Category</span>
          <select
            :value="category ?? ''"
            class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
            data-testid="compare-category"
            @change="selectCategory"
          >
            <option v-if="!category" value="" disabled>Choose a category</option>
            <option v-for="c in categoryOptions" :key="c.slug" :value="c.slug">
              {{ c.name }} (you {{ c.mine_items }} · {{ firstName(other.display_name) }} {{ c.their_items }})
            </option>
          </select>
        </label>

        <template v-if="counts && res.category">
          <div
            role="tablist"
            :aria-label="`Compare ${res.category.name}`"
            class="flex gap-1 overflow-x-auto border-b border-slate-200 dark:border-slate-800"
          >
            <button
              v-for="(b, i) in COMPARE_BUCKETS"
              :id="`${baseId}-tab-${b}`"
              :key="b"
              type="button"
              role="tab"
              :aria-selected="bucket === b ? 'true' : 'false'"
              :aria-controls="`${baseId}-panel`"
              :tabindex="bucket === b ? 0 : -1"
              class="-mb-px inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-sm"
              :class="
                bucket === b
                  ? 'border-slate-900 font-medium text-slate-900 dark:border-slate-100 dark:text-slate-50'
                  : 'border-transparent text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100'
              "
              :data-testid="`compare-tab-${b}`"
              @click="selectBucket(b)"
              @keydown="onTabKey($event, i)"
            >
              {{ bucketLabel(b, other.display_name) }}
              <span
                class="rounded-full bg-slate-200 px-1.5 text-xs font-medium text-slate-700 dark:bg-slate-700 dark:text-slate-200"
                :data-testid="`compare-count-${b}`"
              >{{ counts[b] }}</span>
            </button>
          </div>

          <div
            :id="`${baseId}-panel`"
            role="tabpanel"
            :aria-labelledby="`${baseId}-tab-${bucket}`"
            :aria-busy="refreshing ? 'true' : 'false'"
            class="space-y-3 transition-opacity"
            :class="refreshing ? 'opacity-60' : ''"
          >
            <p v-if="rows.length === 0" :class="empty" data-testid="compare-empty">{{ emptyMessage }}</p>
            <ul v-else class="space-y-2" data-testid="compare-rows">
              <li v-for="row in rows" :key="row.key" :class="card" class="space-y-2" data-testid="compare-row">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="font-medium text-slate-900 dark:text-slate-50">{{ row.name }}</p>
                    <p v-if="row.identifier" class="text-xs text-slate-500 dark:text-slate-400">{{ row.identifier }}</p>
                  </div>
                  <NuxtLink
                    v-if="catalogLink(row)"
                    :to="catalogLink(row)!"
                    class="shrink-0 text-xs"
                    :class="link"
                    data-testid="compare-catalog-link"
                  >
                    Catalog page
                  </NuxtLink>
                </div>

                <dl v-if="row.mine || row.theirs" class="grid gap-1 text-sm text-slate-700 dark:text-slate-300">
                  <div v-if="row.mine" class="flex flex-wrap gap-x-1" data-testid="compare-row-mine">
                    <dt class="font-medium">You:</dt>
                    <dd>
                      {{ sideSummary(row.mine) }} in
                      <NuxtLink :to="`/my/${row.mine.collection_id}`" :class="link">{{ row.mine.collection_name }}</NuxtLink>
                    </dd>
                  </div>
                  <div v-if="row.theirs" class="flex flex-wrap gap-x-1" data-testid="compare-row-theirs">
                    <dt class="font-medium">{{ firstName(other.display_name) }}:</dt>
                    <dd>
                      {{ sideSummary(row.theirs) }} in
                      <NuxtLink :to="`/u/${other.handle}/c/${row.theirs.collection_id}`" :class="link">{{ row.theirs.collection_name }}</NuxtLink>
                    </dd>
                  </div>
                </dl>

                <template v-if="row.entry">
                  <button
                    type="button"
                    :class="secondary"
                    :aria-expanded="openAdd === row.key ? 'true' : 'false'"
                    data-testid="compare-add-toggle"
                    @click="openAdd = openAdd === row.key ? null : row.key"
                  >
                    {{ openAdd === row.key ? "Close" : "Add to my collection" }}
                  </button>
                  <AddToCollection
                    v-if="openAdd === row.key"
                    :entry="row.entry"
                    @added="(c: { id: string; name: string }) => onAdded({ entryName: row.name, collectionName: c.name })"
                  />
                </template>
              </li>
            </ul>

            <p v-if="moreError" role="alert" class="text-sm text-rose-700 dark:text-rose-300">
              We couldn’t load more. Please try again.
            </p>
            <button
              v-if="hasMore"
              type="button"
              :class="secondary"
              :disabled="loadingMore"
              data-testid="compare-show-more"
              @click="loadMore"
            >
              {{ loadingMore ? "Loading…" : "Show more" }}
            </button>
          </div>
        </template>
      </template>
    </template>
  </section>
</template>
