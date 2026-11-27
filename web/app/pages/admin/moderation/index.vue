<script setup lang="ts">
import type { CategoriesResponse } from "~/types/api";

useHead({ title: "Moderation · Collection Tracker" });

const api = useApi();
const { data: categoriesData } = await useAsyncData("categories:all", () =>
  api.get<CategoriesResponse>("/categories"),
);
const categories = computed(() => categoriesData.value?.categories ?? []);
</script>

<template>
  <section class="space-y-4">
    <header>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50">Moderation</h1>
      <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Catalog suggestions and corrections waiting for review, oldest first.
      </p>
    </header>
    <AdminOnly>
      <ModerationQueue :categories="categories" />
    </AdminOnly>
  </section>
</template>
