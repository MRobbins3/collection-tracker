<script setup lang="ts">
const route = useRoute();
const router = useRouter();
const id = route.params.id as string;

const { isSignedIn } = useAuth();
const { data: collection, refresh, pending, error } = useMyCollection(id);
const { rename, remove } = useMyCollectionsActions();

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

async function onDelete() {
  if (!collection.value) return;
  if (!confirm(`Delete “${collection.value.name}”? All items inside are removed too.`)) return;
  await remove(id);
  await router.push("/my");
}
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      to="/my"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800"
    >
      ← My collections
    </NuxtLink>

    <SignInPrompt
      v-if="!isSignedIn"
      message="Sign in to view this collection."
    />

    <template v-else>
      <div v-if="pending && !collection" class="text-sm text-slate-500">Loading…</div>

      <div
        v-else-if="error"
        class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900"
        data-testid="collection-error"
      >
        We couldn’t find this collection.
      </div>

      <article v-else-if="collection" class="space-y-4">
        <header>
          <p class="text-xs uppercase tracking-wide text-slate-500">
            {{ collection.category_name }}
          </p>
          <h1 class="text-2xl font-bold tracking-tight text-slate-900">{{ collection.name }}</h1>
          <p class="mt-1 text-sm text-slate-500">
            {{ collection.item_count }} item{{ collection.item_count === 1 ? "" : "s" }}
          </p>
        </header>

        <form
          class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200"
          @submit.prevent="onRename"
        >
          <label class="block">
            <span class="block text-xs font-medium text-slate-600">Rename this collection</span>
            <input
              v-model="draftName"
              type="text"
              maxlength="100"
              class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900"
              data-testid="rename-collection-input"
            />
          </label>
          <p v-if="renameError" class="text-xs text-rose-700">{{ renameError }}</p>
          <div class="flex justify-between gap-2">
            <button
              type="submit"
              :disabled="!isDirty || saving"
              class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              data-testid="rename-collection-submit"
            >
              {{ saving ? "Saving…" : "Save" }}
            </button>
            <button
              type="button"
              class="inline-flex h-10 items-center justify-center rounded-md border border-rose-300 px-4 text-sm font-medium text-rose-700 hover:bg-rose-50"
              data-testid="delete-collection"
              @click="onDelete"
            >
              Delete collection
            </button>
          </div>
        </form>

        <div class="rounded-xl bg-slate-100 p-4 text-sm text-slate-600">
          Adding items comes next — that’s the next milestone.
        </div>
      </article>
    </template>
  </section>
</template>
