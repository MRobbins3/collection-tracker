<script setup lang="ts">
import { categoryStatusLabel } from "~/utils/categoryBuilder";
import type { Category } from "~/types/api";

defineProps<{ category: Category }>();
</script>

<template>
  <NuxtLink
    :to="`/categories/${category.slug}`"
    class="block rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 transition hover:ring-slate-300 active:ring-slate-400 dark:bg-slate-900 dark:ring-slate-800 dark:hover:ring-slate-700 dark:active:ring-slate-600"
    :data-testid="`category-card-${category.slug}`"
  >
    <div class="flex items-start justify-between gap-4">
      <div class="min-w-0 flex-1">
        <h3 class="text-base font-semibold text-slate-900 dark:text-slate-50">
          {{ category.name }}
          <span
            v-if="categoryStatusLabel(category.status)"
            class="ml-1 inline-block rounded-full bg-amber-100 px-2 py-0.5 align-middle text-xs font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-100"
            :data-testid="`category-status-${category.slug}`"
          >
            {{ categoryStatusLabel(category.status) }}
          </span>
        </h3>
        <p
          v-if="category.description"
          class="mt-1 line-clamp-2 text-sm text-slate-600 dark:text-slate-400"
        >
          {{ category.description }}
        </p>
      </div>
      <span class="shrink-0 text-slate-400 dark:text-slate-500" aria-hidden="true">›</span>
    </div>
  </NuxtLink>
</template>
