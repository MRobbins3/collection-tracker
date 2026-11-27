<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { AttributeSchema } from "~/types/api";

const route = useRoute();
const router = useRouter();
const id = route.params.id as string;

const { isSignedIn } = useAuth();
const { data: collection, refresh, pending, error } = useMyCollection(id);
const { rename, remove: removeCollection } = useMyCollectionsActions();
const { data: itemsData, refresh: refreshItems } = useCollectionItems(id);
const { create, update, remove } = useItemsActions(id);
const { data: category } = useAsyncData<{ attribute_schema: AttributeSchema }>(
  `category-for-collection:${id}`,
  async () => {
    const api = useApi();
    if (!collection.value) return { attribute_schema: { type: "object", properties: {} } };
    return await api.get(`/categories/${collection.value.category_slug}`);
  },
  { watch: [collection] },
);

const categorySchema = computed<AttributeSchema | undefined>(
  () => (category.value?.attribute_schema as AttributeSchema | undefined),
);

const draftName = ref("");
const saving = ref(false);
const renameError = ref<string | null>(null);

watch(
  collection,
  (c) => {
    if (c) draftName.value = c.name;
  },
  { immediate: true },
);

const isDirty = computed(() => collection.value && draftName.value !== collection.value.name);

useHead(() => ({
  title: collection.value ? `${collection.value.name} · Collection Tracker` : "Collection · Collection Tracker",
}));

async function onRename() {
  renameError.value = null;
  const trimmed = draftName.value.trim();
  if (trimmed.length < 1 || trimmed.length > 100) {
    renameError.value = "Name must be 1–100 characters.";
    return;
  }
  saving.value = true;
  try {
    await rename(id, trimmed);
    await refresh();
  } catch (e) {
    renameError.value = "Couldn’t save the new name.";
    console.error(e);
  } finally {
    saving.value = false;
  }
}

async function onDeleteCollection() {
  if (!collection.value) return;
  if (!confirm(`Delete “${collection.value.name}”? All items inside are removed too.`)) return;
  await removeCollection(id);
  await router.push("/my");
}

async function onCreateItem(payload: { name: string; quantity: number; condition?: string; attributes: Record<string, unknown> }) {
  try {
    await create(payload);
    await refreshItems();
    await refresh();
  } catch (e) {
    console.error(e);
    alert("We couldn’t save that item. Check the fields and try again.");
  }
}

async function onUpdateItem(itemId: string, payload: { name: string; quantity: number; condition?: string; attributes: Record<string, unknown> }) {
  try {
    await update(itemId, payload);
    await refreshItems();
  } catch (e) {
    console.error(e);
    alert("We couldn’t save those changes.");
  }
}

async function onDeleteItem(itemId: string, name: string) {
  if (!confirm(`Remove “${name}” from this collection?`)) return;
  await remove(itemId);
  await refreshItems();
  await refresh();
}
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      to="/my"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
    >
      ← My collections
    </NuxtLink>

    <SignInPrompt
      v-if="!isSignedIn"
      message="Sign in to view this collection."
    />

    <template v-else>
      <div v-if="pending && !collection" class="text-sm text-slate-500 dark:text-slate-400">
        Loading…
      </div>

      <div
        v-else-if="error"
        class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
        data-testid="collection-error"
      >
        We couldn’t find this collection.
      </div>

      <article v-else-if="collection" class="space-y-4">
        <header>
          <p class="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">
            {{ collection.category_name }}
          </p>
          <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50">
            {{ collection.name }}
          </h1>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {{ collection.item_count }} item{{ collection.item_count === 1 ? "" : "s" }}
          </p>
        </header>

        <NewItemPanel
          :category-slug="collection.category_slug"
          :schema="categorySchema"
          @create="onCreateItem"
        />

        <ul
          v-if="itemsData && itemsData.items.length > 0"
          class="space-y-2"
          data-testid="items-list"
        >
          <ItemCard
            v-for="it in itemsData.items"
            :key="it.id"
            :item="it"
            :schema="categorySchema"
            @update="(p) => onUpdateItem(it.id, p)"
            @delete="onDeleteItem(it.id, it.name)"
          />
        </ul>
        <div
          v-else
          class="rounded-xl bg-slate-100 p-4 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
          data-testid="items-empty"
        >
          No items yet — add your first one above.
        </div>

        <details
          class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
        >
          <summary class="cursor-pointer text-sm font-medium text-slate-900 dark:text-slate-100">
            Collection settings
          </summary>
          <form class="mt-3 space-y-3" @submit.prevent="onRename">
            <label class="block">
              <span class="block text-xs font-medium text-slate-700 dark:text-slate-300">
                Rename this collection
              </span>
              <input
                v-model="draftName"
                type="text"
                maxlength="100"
                class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
                data-testid="rename-collection-input"
              />
            </label>
            <p v-if="renameError" class="text-xs text-rose-700 dark:text-rose-300">
              {{ renameError }}
            </p>
            <div class="flex justify-between gap-2">
              <button
                type="submit"
                :disabled="!isDirty || saving"
                class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
                data-testid="rename-collection-submit"
              >
                {{ saving ? "Saving…" : "Save" }}
              </button>
              <button
                type="button"
                class="inline-flex h-10 items-center justify-center rounded-md border border-rose-300 px-4 text-sm font-medium text-rose-700 hover:bg-rose-50 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950"
                data-testid="delete-collection"
                @click="onDeleteCollection"
              >
                Delete collection
              </button>
            </div>
          </form>
        </details>
      </article>
    </template>
  </section>
</template>
