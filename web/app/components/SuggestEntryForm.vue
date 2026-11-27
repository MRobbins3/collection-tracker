<script setup lang="ts">
import { computed, reactive, ref, useId } from "vue";
import AttributeFields from "./AttributeFields.vue";
import { apiErrorMessage, outcomeMessage } from "~/utils/moderation";
import { suggestTitleCase } from "~/utils/text";
import type { AttributeSchema, SubmitResult } from "~/types/api";

/**
 * Suggest a new entry for the global catalog (ADR 0012). An admin reviews
 * it; it stays private until approved. The API checks the name and
 * identifier against the catalog (and pending suggestions) and reuses a
 * match instead of creating a duplicate, so the outcome message says which
 * happened. With `itemId`, that item is linked to the entry once the entry
 * is in the catalog.
 */
const props = defineProps<{
  categorySlug: string;
  schema?: AttributeSchema;
  initialName?: string;
  initialAttributes?: Record<string, unknown>;
  itemId?: string;
}>();

const emit = defineEmits<{
  (e: "submitted", result: SubmitResult): void;
  (e: "cancel"): void;
}>();

const { submitEntry } = useSubmissionActions();

const ids = { name: useId(), description: useId() };
const name = ref(props.initialName ?? "");
const description = ref("");
const attrs = reactive<Record<string, unknown>>({ ...(props.initialAttributes ?? {}) });
const busy = ref(false);
const error = ref<string | null>(null);
const result = ref<SubmitResult | null>(null);

const nameSuggestion = computed(() => suggestTitleCase(name.value));

async function onSubmit() {
  error.value = null;
  const trimmed = name.value.trim();
  if (trimmed.length < 1 || trimmed.length > 200) {
    error.value = "Name must be 1–200 characters.";
    return;
  }
  busy.value = true;
  try {
    const res = await submitEntry({
      category_slug: props.categorySlug,
      name: trimmed,
      description: description.value.trim() || undefined,
      attributes: Object.fromEntries(Object.entries(attrs).filter(([, v]) => v !== undefined && v !== "")),
      item_id: props.itemId,
    });
    result.value = res;
    emit("submitted", res);
  } catch (e) {
    console.error(e);
    error.value = apiErrorMessage(e, "Couldn’t send your suggestion. Please try again.");
  } finally {
    busy.value = false;
  }
}

const inputClass =
  "mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400";
</script>

<template>
  <div class="space-y-3" data-testid="suggest-entry">
    <div
      v-if="result"
      role="status"
      class="rounded-md bg-emerald-50 p-3 text-sm text-emerald-900 ring-1 ring-emerald-200 dark:bg-emerald-950 dark:text-emerald-100 dark:ring-emerald-800"
      data-testid="suggest-entry-result"
    >
      <p>{{ outcomeMessage(result) }}</p>
      <p class="mt-1 text-xs">
        <NuxtLink to="/my/submissions" class="underline">My submissions</NuxtLink>
      </p>
    </div>

    <form v-else class="space-y-3" @submit.prevent="onSubmit">
      <p class="text-xs text-slate-600 dark:text-slate-400">
        An admin reviews every suggestion before it appears in the catalog. Only you can see it until then.
      </p>
      <div>
        <label :for="ids.name" class="block text-xs font-medium text-slate-700 dark:text-slate-300">Name</label>
        <input
          :id="ids.name"
          v-model="name"
          type="text"
          maxlength="200"
          :class="inputClass"
          data-testid="suggest-entry-name"
        >
        <p v-if="nameSuggestion" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
          Tip: catalog names usually look best in Title Case.
          <button
            type="button"
            class="font-medium text-slate-900 underline underline-offset-2 dark:text-slate-100"
            data-testid="suggest-entry-title-case"
            @click="name = nameSuggestion ?? name"
          >
            Use “{{ nameSuggestion }}”
          </button>
        </p>
      </div>

      <AttributeFields
        v-if="schema"
        :schema="schema"
        :model-value="attrs"
        @update:model-value="Object.assign(attrs, $event)"
      />

      <div>
        <label :for="ids.description" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
          Description <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
        </label>
        <textarea
          :id="ids.description"
          v-model="description"
          rows="2"
          maxlength="1000"
          :class="inputClass"
          data-testid="suggest-entry-description"
        />
      </div>

      <p v-if="error" role="alert" class="text-xs text-rose-700 dark:text-rose-300" data-testid="suggest-entry-error">
        {{ error }}
      </p>

      <div class="flex gap-2">
        <button
          type="submit"
          :disabled="busy"
          class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
          data-testid="suggest-entry-submit"
        >
          {{ busy ? "Sending…" : "Suggest for the catalog" }}
        </button>
        <button
          type="button"
          class="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
          @click="emit('cancel')"
        >
          Cancel
        </button>
      </div>
    </form>
  </div>
</template>
