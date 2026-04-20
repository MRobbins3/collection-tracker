<script setup lang="ts">
const route = useRoute();
const slug = computed(() => route.params.slug as string);
const { data: category, pending, error } = useCategory(slug.value);

useHead(() => ({
  title: category.value ? `${category.value.name} · Collection Tracker` : "Category · Collection Tracker",
}));

interface SchemaProperty {
  title?: string;
  description?: string;
}

interface SchemaWithProps {
  properties?: Record<string, SchemaProperty>;
}

const attributeLabels = computed<{ key: string; title: string; description?: string }[]>(() => {
  const props = (category.value?.attribute_schema as SchemaWithProps | undefined)?.properties;
  if (!props) return [];
  return Object.entries(props).map(([key, def]) => ({
    key,
    title: def.title ?? humanize(key),
    description: def.description,
  }));
});

function humanize(key: string): string {
  const spaced = key.replace(/_/g, " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      to="/categories"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
    >
      ← All categories
    </NuxtLink>

    <div
      v-if="pending"
      class="text-sm text-slate-500 dark:text-slate-400"
      data-testid="category-loading"
    >
      Loading…
    </div>
    <div
      v-else-if="error"
      class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      data-testid="category-error"
    >
      We couldn’t find this category.
    </div>
    <article v-else-if="category" class="space-y-4">
      <header>
        <h1
          class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50"
          data-testid="category-name"
        >
          {{ category.name }}
        </h1>
        <p v-if="category.description" class="mt-1 text-slate-600 dark:text-slate-400">
          {{ category.description }}
        </p>
      </header>

      <section
        v-if="attributeLabels.length > 0"
        class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
      >
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
          What you can track for each item
        </h2>
        <dl class="mt-3 space-y-3">
          <div v-for="a in attributeLabels" :key="a.key">
            <dt class="text-sm font-medium text-slate-900 dark:text-slate-100">{{ a.title }}</dt>
            <dd v-if="a.description" class="mt-0.5 text-sm text-slate-600 dark:text-slate-400">
              {{ a.description }}
            </dd>
          </div>
        </dl>
      </section>

      <div
        class="rounded-xl bg-slate-100 p-4 text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      >
        Sign in to start a collection in <strong>{{ category.name }}</strong>. (Sign-in coming soon.)
      </div>
    </article>
  </section>
</template>
