<script setup lang="ts">
import { computed, ref, useId } from "vue";
import { categoryStatusLabel } from "~/utils/categoryBuilder";
import { apiErrorMessage } from "~/utils/moderation";
import type { Category } from "~/types/api";

/**
 * Shown to the creator of a category on its page (ADR 0013): where it
 * stands, why a reviewer turned it down, and the next step (edit, or ask for
 * it to join the public list).
 */
const props = defineProps<{ category: Category }>();
const emit = defineEmits<{ (e: "changed"): void }>();

const { submit } = useMyCategoryActions();
const noteId = useId();
const asking = ref(false);
const note = ref("");
const busy = ref(false);
const error = ref<string | null>(null);

const label = computed(() => categoryStatusLabel(props.category.status) ?? "On the public list");
const canChange = computed(() => props.category.status === "private" || props.category.status === "rejected");

const explanation = computed(() => {
  switch (props.category.status) {
    case "private":
      return "Only you, and people who can see your collections, can see it. You can use it right away.";
    case "pending":
      return "An admin will look at it soon. You can keep using it meanwhile; it stays private until approved.";
    case "rejected":
      return "It stays private, and you can keep using it. Change it and ask again if you like.";
    default:
      return "Everyone can find it in the category list. Changes now go through review.";
  }
});

async function onSubmit() {
  busy.value = true;
  error.value = null;
  try {
    await submit(props.category.slug, note.value.trim() || undefined);
    asking.value = false;
    note.value = "";
    emit("changed");
  } catch (e) {
    console.error(e);
    error.value = apiErrorMessage(e, "That didn’t work. Please try again.");
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <section
    class="space-y-3 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="category-owner-panel"
  >
    <div class="flex flex-wrap items-center gap-2">
      <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">You built this category</h2>
      <span
        class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-100"
        data-testid="category-status"
      >
        {{ label }}
      </span>
    </div>
    <p class="text-sm text-slate-600 dark:text-slate-400">{{ explanation }}</p>
    <p
      v-if="category.status === 'rejected' && category.review_reason"
      class="rounded-md bg-slate-50 p-3 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
      data-testid="category-review-reason"
    >
      <span class="font-medium">Reviewer’s note:</span> {{ category.review_reason }}
    </p>

    <div v-if="canChange && !asking" class="flex flex-wrap gap-2">
      <NuxtLink
        :to="`/categories/${category.slug}/edit`"
        class="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
        data-testid="category-edit"
      >
        Edit
      </NuxtLink>
      <button
        type="button"
        class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        data-testid="category-ask-public"
        @click="asking = true"
      >
        Submit for the public list
      </button>
    </div>

    <form v-if="asking" class="space-y-2" @submit.prevent="onSubmit">
      <label :for="noteId" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
        Anything the reviewer should know? <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
      </label>
      <textarea
        :id="noteId"
        v-model="note"
        rows="2"
        maxlength="500"
        class="block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        data-testid="category-ask-note"
      />
      <div class="flex gap-2">
        <button
          type="submit"
          :disabled="busy"
          class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
          data-testid="category-ask-submit"
        >
          Send for review
        </button>
        <button
          type="button"
          class="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
          @click="asking = false"
        >
          Cancel
        </button>
      </div>
    </form>
    <p v-if="error" role="alert" class="text-sm text-rose-700 dark:text-rose-300">{{ error }}</p>
  </section>
</template>
