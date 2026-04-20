<script setup lang="ts">
const route = useRoute();
const slug = computed(() => route.params.slug as string);
const { data: category, pending, error } = useCategory(slug.value);

useHead(() => ({
  title: category.value ? `${category.value.name} · Collection Tracker` : "Category · Collection Tracker",
}));

const attributeKeys = computed<string[]>(() => {
  const props = (category.value?.attribute_schema as { properties?: Record<string, unknown> } | undefined)?.properties;
  return props ? Object.keys(props) : [];
});
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      to="/categories"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800"
    >
      ← All categories
    </NuxtLink>

    <div v-if="pending" class="text-sm text-slate-500" data-testid="category-loading">Loading…</div>
    <div
      v-else-if="error"
      class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900"
      data-testid="category-error"
    >
      We couldn’t find this category.
    </div>
    <article v-else-if="category" class="space-y-4">
      <header>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900" data-testid="category-name">
          {{ category.name }}
        </h1>
        <p v-if="category.description" class="mt-1 text-slate-600">{{ category.description }}</p>
      </header>

      <section class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Category-specific fields
        </h2>
        <ul v-if="attributeKeys.length > 0" class="mt-2 flex flex-wrap gap-2">
          <li
            v-for="key in attributeKeys"
            :key="key"
            class="rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-700"
          >
            {{ key }}
          </li>
        </ul>
        <p v-else class="mt-2 text-sm text-slate-500">No extra fields for this category.</p>
      </section>

      <div class="rounded-xl bg-slate-100 p-4 text-sm text-slate-600">
        Sign in to start a collection in <strong>{{ category.name }}</strong>. (Sign-in coming soon.)
      </div>
    </article>
  </section>
</template>
