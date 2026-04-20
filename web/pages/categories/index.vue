<script setup lang="ts">
const query = ref("");
const { data, pending, error } = useCategoriesList(query);

useHead({ title: "Browse categories · Collection Tracker" });
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-xl font-bold tracking-tight text-slate-900">Categories</h1>
      <p class="mt-1 text-sm text-slate-600">Find the kind of thing you collect.</p>
    </div>

    <label class="block">
      <span class="sr-only">Search categories</span>
      <input
        v-model="query"
        type="search"
        inputmode="search"
        placeholder="Search categories…"
        class="block w-full rounded-lg border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900"
        data-testid="category-search"
      />
    </label>

    <div v-if="pending && !data" class="text-sm text-slate-500" data-testid="categories-loading">
      Loading…
    </div>
    <div
      v-else-if="error"
      class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900"
      data-testid="categories-error"
    >
      Couldn’t load categories. Please try again.
    </div>
    <ul
      v-else-if="data && data.categories.length > 0"
      class="space-y-2"
      data-testid="categories-list"
    >
      <li v-for="cat in data.categories" :key="cat.id">
        <CategoryCard :category="cat" />
      </li>
    </ul>
    <div
      v-else
      class="rounded-lg bg-slate-100 p-4 text-sm text-slate-600"
      data-testid="categories-empty"
    >
      No categories match “{{ query }}”.
    </div>
  </section>
</template>
