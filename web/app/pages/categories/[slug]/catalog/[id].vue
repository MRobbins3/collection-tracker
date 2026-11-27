<script setup lang="ts">
import { computed } from "vue";
import { humanizeKey, identifierTitle } from "~/utils/catalog";
import type { AttributeSchema } from "~/types/api";

const route = useRoute();
const slug = route.params.slug as string;
const id = route.params.id as string;

const { data: category } = useCategory(slug);
const { data: entry, pending, error } = useCatalogEntry(id);

// An entry reached through another category's URL is treated as missing.
const found = computed(() => (entry.value && entry.value.category_slug === slug ? entry.value : null));
const idTitle = computed(() => identifierTitle(category.value));

const details = computed<{ key: string; title: string; value: string }[]>(() => {
  if (!found.value) return [];
  const props = (category.value?.attribute_schema as Partial<AttributeSchema> | undefined)?.properties ?? {};
  // Schema order first, so every entry in a category reads the same way.
  const keys = [
    ...Object.keys(props),
    ...Object.keys(found.value.attributes).filter((k) => !(k in props)),
  ];
  // The identifier is already shown under the name, so it isn't repeated here.
  const shownAbove = category.value?.catalog_identifier;
  return keys
    .filter((key) => key !== shownAbove)
    .map((key) => ({ key, value: found.value!.attributes[key] }))
    .filter(({ value }) => value !== undefined && value !== null && value !== "")
    .map(({ key, value }) => ({ key, title: props[key]?.title ?? humanizeKey(key), value: String(value) }));
});

useHead(() => ({
  title: found.value ? `${found.value.name} · Collection Tracker` : "Catalog · Collection Tracker",
}));
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      :to="`/categories/${slug}`"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
    >
      ← {{ category?.name ?? "Back to category" }}
    </NuxtLink>

    <div v-if="pending && !entry" class="text-sm text-slate-500 dark:text-slate-400" data-testid="catalog-entry-loading">
      Loading…
    </div>
    <div
      v-else-if="error || !found"
      class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      data-testid="catalog-entry-error"
    >
      We couldn’t find this catalog entry.
    </div>
    <article v-else class="space-y-4">
      <header>
        <p class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
          {{ found.category_name }} catalog
        </p>
        <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50" data-testid="catalog-entry-name">
          {{ found.name }}
        </h1>
        <p v-if="idTitle && found.identifier" class="mt-0.5 text-sm text-slate-600 dark:text-slate-400">
          {{ idTitle }} {{ found.identifier }}
        </p>
        <p v-if="found.description" class="mt-2 text-slate-700 dark:text-slate-300">{{ found.description }}</p>
      </header>

      <section
        v-if="details.length > 0"
        class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
      >
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Details</h2>
        <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-sm" data-testid="catalog-entry-details">
          <template v-for="d in details" :key="d.key">
            <dt class="text-slate-500 dark:text-slate-400">{{ d.title }}</dt>
            <dd class="text-slate-900 dark:text-slate-100">{{ d.value }}</dd>
          </template>
        </dl>
      </section>

      <AddToCollection :entry="found" />
    </article>
  </section>
</template>
