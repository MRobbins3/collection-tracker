<script setup lang="ts">
import { apiErrorMessage } from "~/utils/moderation";
import type { CategoryDraft } from "~/types/api";

useHead({ title: "Create a category · Collection Tracker" });

const { isSignedIn } = useAuth();
const { data: library, status } = useCategoryFields();
const { create } = useMyCategoryActions();

const busy = ref(false);
const error = ref<string | null>(null);
const existing = ref<{ slug: string; name: string } | null>(null);

async function onSave(draft: CategoryDraft) {
  busy.value = true;
  error.value = null;
  existing.value = null;
  try {
    const cat = await create(draft);
    await navigateTo(`/categories/${cat.slug}`);
  } catch (e) {
    console.error(e);
    error.value = apiErrorMessage(e, "We couldn’t save your category. Please try again.");
    existing.value = (e as { data?: { existing?: { slug: string; name: string } } })?.data?.existing ?? null;
  } finally {
    busy.value = false;
  }
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
    <header>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50">Create a category</h1>
      <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Can’t find what you collect? Build it from fields other categories use. It’s private to you (and people
        who can see your collections) until you ask for it to join the public list.
      </p>
    </header>
    <SignInPrompt v-if="!isSignedIn" message="Sign in to create a category." />
    <CategoryBuilder
      v-else
      :library="library"
      :library-loading="status === 'pending' || status === 'idle'"
      :busy="busy"
      :error="error"
      :existing="existing"
      submit-label="Create category"
      @save="onSave"
      @cancel="navigateTo('/categories')"
    />
  </section>
</template>
