<script setup lang="ts">
import { reactive, ref, useId } from "vue";
import AttributeFields from "./AttributeFields.vue";
import { apiErrorMessage } from "~/utils/moderation";
import type { AttributeSchema, CatalogEntry } from "~/types/api";
import type { CorrectionInput } from "~/composables/useModeration";

/**
 * Propose a correction to an approved catalog entry (ADR 0012). Only the
 * fields you change are sent; nothing in the catalog changes until an
 * admin approves.
 */
const props = defineProps<{
  entry: CatalogEntry;
  schema?: AttributeSchema;
}>();

const { proposeCorrection } = useSubmissionActions();

const ids = { name: useId(), description: useId(), note: useId() };
const open = ref(false);
const name = ref(props.entry.name);
const description = ref(props.entry.description ?? "");
const note = ref("");
const attrs = reactive<Record<string, unknown>>({ ...props.entry.attributes });
const busy = ref(false);
const error = ref<string | null>(null);
const sent = ref(false);

function cleanAttrs(a: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(a).filter(([, v]) => v !== undefined && v !== null && v !== ""));
}

function sameAttrs(a: Record<string, unknown>, b: Record<string, unknown>): boolean {
  const ka = Object.keys(a);
  return ka.length === Object.keys(b).length && ka.every((k) => String(a[k]) === String(b[k]));
}

async function onSubmit() {
  error.value = null;
  const input: CorrectionInput = { catalog_entry_id: props.entry.id };
  const n = name.value.trim();
  if (n !== props.entry.name) {
    if (n.length < 1 || n.length > 200) {
      error.value = "Name must be 1–200 characters.";
      return;
    }
    input.name = n;
  }
  const d = description.value.trim();
  if (d !== (props.entry.description ?? "")) input.description = d;
  const a = cleanAttrs(attrs);
  if (!sameAttrs(a, cleanAttrs(props.entry.attributes))) input.attributes = a;
  if (input.name === undefined && input.description === undefined && input.attributes === undefined) {
    error.value = "Change at least one detail first.";
    return;
  }
  if (note.value.trim()) input.note = note.value.trim();

  busy.value = true;
  try {
    await proposeCorrection(input);
    sent.value = true;
    open.value = false;
  } catch (e) {
    console.error(e);
    error.value = apiErrorMessage(e, "Couldn’t send your correction. Please try again.");
  } finally {
    busy.value = false;
  }
}

const inputClass =
  "mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400";
</script>

<template>
  <section
    class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="correction"
  >
    <div
      v-if="sent"
      role="status"
      class="text-sm text-emerald-900 dark:text-emerald-100"
      data-testid="correction-sent"
    >
      Thanks! An admin will review your correction; the catalog keeps the current details until then.
      <NuxtLink to="/my/submissions" class="underline">My submissions</NuxtLink>
    </div>

    <button
      v-else-if="!open"
      type="button"
      class="inline-flex items-center gap-1 text-sm font-medium text-slate-700 hover:text-slate-900 dark:text-slate-200 dark:hover:text-white"
      data-testid="correction-open"
      @click="open = true"
    >
      <Icon name="lucide:pencil" class="h-4 w-4" aria-hidden="true" />
      Suggest a correction
    </button>

    <form v-else class="space-y-3" @submit.prevent="onSubmit">
      <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Suggest a correction</h2>
      <p class="text-xs text-slate-600 dark:text-slate-400">
        Fix anything that’s wrong or missing. An admin reviews it before the catalog changes.
      </p>
      <div>
        <label :for="ids.name" class="block text-xs font-medium text-slate-700 dark:text-slate-300">Name</label>
        <input :id="ids.name" v-model="name" type="text" maxlength="200" :class="inputClass" data-testid="correction-name">
      </div>
      <AttributeFields
        v-if="schema"
        :schema="schema"
        :model-value="attrs"
        @update:model-value="Object.assign(attrs, $event)"
      />
      <div>
        <label :for="ids.description" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
          Description
        </label>
        <textarea
          :id="ids.description"
          v-model="description"
          rows="2"
          maxlength="1000"
          :class="inputClass"
          data-testid="correction-description"
        />
      </div>
      <div>
        <label :for="ids.note" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
          Note for the reviewer <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
        </label>
        <input
          :id="ids.note"
          v-model="note"
          type="text"
          maxlength="500"
          placeholder="Where did you find this?"
          :class="inputClass"
          data-testid="correction-note"
        >
      </div>
      <p v-if="error" role="alert" class="text-xs text-rose-700 dark:text-rose-300" data-testid="correction-error">
        {{ error }}
      </p>
      <div class="flex gap-2">
        <button
          type="submit"
          :disabled="busy"
          class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
          data-testid="correction-submit"
        >
          {{ busy ? "Sending…" : "Send correction" }}
        </button>
        <button
          type="button"
          class="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
          @click="open = false"
        >
          Cancel
        </button>
      </div>
    </form>
  </section>
</template>
