<script setup lang="ts">
import { apiErrorMessage } from "~/utils/moderation";
import type { CategoryDraft } from "~/types/api";

const route = useRoute();
const slug = route.params.slug as string;
const { isSignedIn, ready } = useAuth();
const { data: category, pending } = useCategory(slug);
const { data: library, status: libraryStatus } = useCategoryFields();
const { update } = useMyCategoryActions();

useHead(() => ({ title: category.value ? `Edit ${category.value.name} · Collection Tracker` : "Edit category · Collection Tracker" }));

const editable = computed(
  () => !!category.value?.mine && (category.value.status === "private" || category.value.status === "rejected"),
);
const libraryReady = computed(() => libraryStatus.value === "success" || libraryStatus.value === "error");

const busy = ref(false);
const error = ref<string | null>(null);
const existing = ref<{ slug: string; name: string } | null>(null);

async function onSave(draft: CategoryDraft) {
  busy.value = true;
  error.value = null;
  existing.value = null;
  try {
    await update(slug, draft);
    await navigateTo(`/categories/${slug}`);
  } catch (e) {
    console.error(e);
    error.value = apiErrorMessage(e, "We couldn’t save your changes. Please try again.");
    existing.value = (e as { data?: { existing?: { slug: string; name: string } } })?.data?.existing ?? null;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      :to="`/categories/${slug}`"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
    >
      ← Back to the category
    </NuxtLink>
    <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50">Edit category</h1>
    <SignInPrompt v-if="ready && !isSignedIn" message="Sign in to edit your categories." />
    <p v-else-if="!ready || pending || !libraryReady" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
    <div
      v-else-if="!editable"
      class="rounded-lg bg-slate-100 p-4 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
      data-testid="category-not-editable"
    >
      You can only edit a category you built, and not while it waits for review or once it’s on the public list.
    </div>
    <CategoryBuilder
      v-else-if="category"
      :library="library"
      :initial="category"
      :busy="busy"
      :error="error"
      :existing="existing"
      submit-label="Save changes"
      @save="onSave"
      @cancel="navigateTo(`/categories/${slug}`)"
    />
  </section>
</template>
