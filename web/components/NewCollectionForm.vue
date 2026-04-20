<script setup lang="ts">
import { ref } from "vue";
import type { Category, Collection } from "~/types/api";

const props = defineProps<{ categories: Category[] }>();
const emit = defineEmits<{ (e: "created", collection: Collection): void }>();

const { create } = useMyCollectionsActions();

const categorySlug = ref<string>(props.categories[0]?.slug ?? "");
const name = ref("");
const submitting = ref(false);
const error = ref<string | null>(null);

async function onSubmit() {
  error.value = null;
  const trimmed = name.value.trim();
  if (trimmed.length < 1 || trimmed.length > 100) {
    error.value = "Give your collection a name (1–100 characters).";
    return;
  }
  if (!categorySlug.value) {
    error.value = "Pick a category.";
    return;
  }

  submitting.value = true;
  try {
    const created = await create({ categorySlug: categorySlug.value, name: trimmed });
    emit("created", created);
    name.value = "";
  } catch (e) {
    error.value = "Couldn’t create that collection. Please try again.";
    console.error(e);
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <form
    class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="new-collection-form"
    @submit.prevent="onSubmit"
  >
    <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Start a new collection</h2>

    <label class="block">
      <span class="block text-xs font-medium text-slate-600 dark:text-slate-400">Category</span>
      <select
        v-model="categorySlug"
        class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        data-testid="new-collection-category"
      >
        <option v-for="c in categories" :key="c.id" :value="c.slug">{{ c.name }}</option>
      </select>
    </label>

    <label class="block">
      <span class="block text-xs font-medium text-slate-600 dark:text-slate-400">Name</span>
      <input
        v-model="name"
        type="text"
        maxlength="100"
        placeholder="e.g. My Star Wars Lego"
        class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        data-testid="new-collection-name"
      />
    </label>

    <p
      v-if="error"
      class="text-xs text-rose-700 dark:text-rose-300"
      data-testid="new-collection-error"
    >
      {{ error }}
    </p>

    <button
      type="submit"
      :disabled="submitting"
      class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
      data-testid="new-collection-submit"
    >
      {{ submitting ? "Creating…" : "Create collection" }}
    </button>
  </form>
</template>
