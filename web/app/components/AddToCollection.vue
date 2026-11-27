<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { CatalogEntry, Collection } from "~/types/api";

/**
 * "Add to my collection" for one catalog entry: pick one of your collections
 * in the entry's category, or start a new one, and add an item linked to
 * the entry with its details filled in. Also says where you already have it.
 */
const props = defineProps<{ entry: CatalogEntry }>();
// "added" fires after the item is saved, with the collection it went into.
const emit = defineEmits<{ (e: "added", collection: { id: string; name: string }): void }>();

const { isSignedIn } = useAuth();
const { data: collectionsData, pending: collectionsPending, refresh: refreshCollections } = useMyCollections();
const { data: ownership, refresh: refreshOwnership } = useCatalogOwnership(props.entry.category_slug);
const { create: createCollection } = useMyCollectionsActions();
const { addEntry } = useCatalogActions();

const NEW = "__new__";

const collections = computed<Collection[]>(() =>
  (collectionsData.value?.collections ?? []).filter((c) => c.category_slug === props.entry.category_slug),
);
const ownedIn = computed(() =>
  (ownership.value?.owned ?? []).filter((o) => o.catalog_entry_id === props.entry.id),
);

const target = ref<string>(NEW);
const newName = ref(props.entry.category_name);
const busy = ref(false);
const error = ref<string | null>(null);
const added = ref<{ id: string; name: string } | null>(null);

// Default to your first collection in this category once they load.
watch(
  collections,
  (list) => {
    if (target.value === NEW && list.length > 0) target.value = list[0]!.id;
  },
  { immediate: true },
);

async function onSubmit() {
  error.value = null;
  added.value = null;
  busy.value = true;
  try {
    let collection: { id: string; name: string } | undefined;
    if (target.value === NEW) {
      const name = newName.value.trim();
      if (name.length < 1 || name.length > 100) {
        error.value = "Collection names are 1–100 characters.";
        return;
      }
      collection = await createCollection({ categorySlug: props.entry.category_slug, name });
      // Select it right away, so a retry after a failed add reuses it
      // instead of creating a second collection.
      await refreshCollections();
      target.value = collection.id;
    } else {
      collection = collections.value.find((c) => c.id === target.value);
    }
    if (!collection) return;
    await addEntry(collection.id, props.entry);
    added.value = { id: collection.id, name: collection.name };
    await Promise.all([refreshOwnership(), refreshCollections()]);
    emit("added", { id: collection.id, name: collection.name });
  } catch (e) {
    console.error(e);
    error.value = "Couldn’t add it to your collection. Please try again.";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <section
    class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="add-to-collection"
  >
    <SignInPrompt v-if="!isSignedIn" :message="`Sign in to add ${entry.name} to your collection.`" />

    <template v-else>
      <p
        v-if="ownedIn.length > 0"
        class="flex flex-wrap items-center gap-1 text-sm text-emerald-800 dark:text-emerald-200"
        data-testid="add-to-collection-owned"
      >
        <Icon name="lucide:check" class="h-4 w-4" aria-hidden="true" />
        <span>In your collection:</span>
        <template v-for="(o, i) in ownedIn" :key="o.item_id">
          <NuxtLink :to="`/my/${o.collection_id}`" class="font-medium underline">{{ o.collection_name }}</NuxtLink><span v-if="i < ownedIn.length - 1">,</span>
        </template>
      </p>

      <div v-if="collectionsPending && !collectionsData?.collections?.length" class="text-sm text-slate-500 dark:text-slate-400">
        Loading your collections…
      </div>

      <form v-else class="space-y-3" @submit.prevent="onSubmit">
        <fieldset class="space-y-2">
          <legend class="text-sm font-semibold text-slate-900 dark:text-slate-100">
            {{ ownedIn.length > 0 ? "Add another copy to" : "Add to my collection" }}
          </legend>
          <label
            v-for="c in collections"
            :key="c.id"
            class="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300"
          >
            <input v-model="target" type="radio" name="add-to-collection-target" :value="c.id">
            {{ c.name }}
            <span class="text-xs text-slate-500 dark:text-slate-400">· {{ c.item_count }} item{{ c.item_count === 1 ? "" : "s" }}</span>
          </label>
          <label class="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
            <input v-model="target" type="radio" name="add-to-collection-target" :value="NEW" data-testid="add-to-collection-new">
            A new collection
          </label>
          <label v-if="target === NEW" class="block pl-6">
            <span class="block text-xs font-medium text-slate-700 dark:text-slate-300">New collection name</span>
            <input
              v-model="newName"
              type="text"
              maxlength="100"
              class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
              data-testid="add-to-collection-name"
            >
          </label>
        </fieldset>

        <p v-if="error" role="alert" class="text-sm text-rose-700 dark:text-rose-300">{{ error }}</p>
        <p v-if="added" role="status" class="text-sm text-emerald-800 dark:text-emerald-200" data-testid="add-to-collection-added">
          Added to <NuxtLink :to="`/my/${added.id}`" class="font-medium underline">{{ added.name }}</NuxtLink>.
        </p>

        <button
          type="submit"
          :disabled="busy"
          class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
          data-testid="add-to-collection-submit"
        >
          {{ busy ? "Adding…" : "Add to my collection" }}
        </button>
      </form>
    </template>
  </section>
</template>
