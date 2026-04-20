<script setup lang="ts">
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
        <h3 class="text-base font-semibold text-slate-900 dark:text-slate-50">{{ category.name }}</h3>
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
