<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import type { Category, OverviewCategory } from "~/types/api";
import PinCategoryButton from "./PinCategoryButton.vue";
import { categoryStatusLabel } from "~/utils/categoryBuilder";

/**
 * Inline panel on the overview listing every global category, so you can add
 * one to your collection before you own anything in it. Inline rather than a
 * modal dialog: on a phone it reads as part of the page and needs no focus
 * trap. Focus moves to its heading when it opens.
 */
const props = defineProps<{
  categories: Category[];
  overview: OverviewCategory[];
  busySlug: string | null;
  error: string | null;
}>();
const emit = defineEmits<{
  pin: [slug: string];
  unpin: [slug: string];
  close: [];
}>();

const heading = ref<HTMLElement | null>(null);
onMounted(() => heading.value?.focus());

const bySlug = computed(() => new Map(props.overview.map((c) => [c.slug, c])));

function stateOf(slug: string): "none" | "pinned" | "collecting" {
  const c = bySlug.value.get(slug);
  if (!c) return "none";
  return c.collection_count > 0 ? "collecting" : "pinned";
}
</script>

<template>
  <section
    aria-labelledby="add-category-heading"
    class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="add-category-panel"
  >
    <div class="flex items-start justify-between gap-3">
      <div>
        <h2
          id="add-category-heading"
          ref="heading"
          tabindex="-1"
          class="text-sm font-semibold text-slate-900 focus:outline-none dark:text-slate-100"
        >
          Add a category
        </h2>
        <p class="mt-0.5 text-xs text-slate-600 dark:text-slate-400">
          Show a category in your collection, even before you add anything to it.
        </p>
      </div>
      <button
        type="button"
        class="shrink-0 rounded-md p-1 text-slate-500 hover:bg-slate-100 hover:text-slate-800 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
        aria-label="Close"
        data-testid="add-category-close"
        @click="emit('close')"
      >
        <Icon name="lucide:x" class="h-5 w-5" aria-hidden="true" />
      </button>
    </div>

    <p
      v-if="error"
      role="alert"
      class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
    >
      {{ error }}
    </p>

    <ul class="divide-y divide-slate-100 dark:divide-slate-800">
      <li
        v-for="c in categories"
        :key="c.id"
        class="flex items-center justify-between gap-3 py-2"
        data-testid="add-category-row"
      >
        <div class="min-w-0">
          <p class="text-sm font-medium text-slate-900 dark:text-slate-100">
            {{ c.name }}
            <span
              v-if="categoryStatusLabel(c.status)"
              class="ml-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-100"
            >
              {{ categoryStatusLabel(c.status) }}
            </span>
          </p>
          <p v-if="c.description" class="truncate text-xs text-slate-500 dark:text-slate-400">
            {{ c.description }}
          </p>
        </div>
        <PinCategoryButton
          :slug="c.slug"
          :name="c.name"
          :state="stateOf(c.slug)"
          :busy="busySlug === c.slug"
          @add="emit('pin', c.slug)"
          @remove="emit('unpin', c.slug)"
        />
      </li>
    </ul>

    <p class="border-t border-slate-100 pt-3 text-sm text-slate-600 dark:border-slate-800 dark:text-slate-400">
      Don’t see what you collect?
      <NuxtLink to="/categories/new" class="font-medium text-slate-900 underline dark:text-slate-100" data-testid="add-category-create">
        Create a category
      </NuxtLink>
    </p>
  </section>
</template>
