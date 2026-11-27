<script setup lang="ts">
import { computed, reactive, ref, useId } from "vue";
import AttributeFields from "./AttributeFields.vue";
import {
  CATEGORY_DESCRIPTION_MAX,
  CATEGORY_NAME_MAX,
  addOption,
  draftProblem,
  groupFieldsBySource,
  previewSchema,
  selectionsFromCategory,
  toDraft,
  type FieldSelection,
} from "~/utils/categoryBuilder";
import { suggestTitleCase } from "~/utils/text";
import type { Category, CategoryDraft, LibraryField } from "~/types/api";

/**
 * Build (or edit) a category from fields other categories already use
 * (ADR 0013): pick fields from the library, add your own options to text
 * fields, preview the item form, save. The page does the saving; this
 * component only produces the draft.
 */
const props = defineProps<{
  library: LibraryField[];
  libraryLoading?: boolean;
  /** The category being edited; absent when creating. */
  initial?: Category | null;
  busy?: boolean;
  error?: string | null;
  /** A category you can already use with the same name (from a 409). */
  existing?: { slug: string; name: string } | null;
  submitLabel?: string;
}>();

const emit = defineEmits<{
  (e: "save", draft: CategoryDraft): void;
  (e: "cancel"): void;
}>();

const ids = { name: useId(), description: useId(), search: useId(), identifier: useId() };

const name = ref(props.initial?.name ?? "");
const description = ref(props.initial?.description ?? "");
const selections = ref<FieldSelection[]>(props.initial ? selectionsFromCategory(props.initial, props.library) : []);
const identifierId = ref(
  props.initial?.catalog_identifier
    ? (selections.value.find((s) => s.field.key === props.initial?.catalog_identifier)?.field.id ?? "")
    : "",
);
const search = ref("");
const localError = ref<string | null>(null);

const nameSuggestion = computed(() => suggestTitleCase(name.value));
const groups = computed(() => groupFieldsBySource(props.library, search.value));
const selectedIds = computed(() => new Set(selections.value.map((s) => s.field.id)));
const schema = computed(() => previewSchema(selections.value));
const previewValues = ref<Record<string, unknown>>({});

// Per-field option input and feedback, keyed by field id.
const optionInput = reactive<Record<string, string>>({});
const optionFeedback = reactive<Record<string, string>>({});

function typeLabel(f: LibraryField): string {
  return f.type === "string" ? "Text" : "Number";
}

/** Where else a field is used: up to three category names, then a count. */
function sourceNames(f: LibraryField): string {
  const names = f.sources.map((s) => s.name);
  if (names.length <= 3) return names.join(", ");
  return `${names.slice(0, 3).join(", ")}, and ${names.length - 3} more`;
}

/** Adds a field, starting with the options the category you picked it from offers. */
function addField(f: LibraryField, sourceSlug: string) {
  if (selectedIds.value.has(f.id)) return;
  const options = f.sources.find((s) => s.slug === sourceSlug)?.options ?? [];
  selections.value = [...selections.value, { field: f, options: [...options] }];
  localError.value = null;
}

function removeField(id: string) {
  selections.value = selections.value.filter((s) => s.field.id !== id);
  if (identifierId.value === id) identifierId.value = "";
}

function typedOption(id: string): string {
  return optionInput[id] ?? "";
}

function optionSuggestion(id: string): string | null {
  return suggestTitleCase(typedOption(id));
}

function commitOption(sel: FieldSelection, value: string) {
  const res = addOption(sel.options, value);
  if (res.kind === "added") {
    sel.options = res.options;
    optionInput[sel.field.id] = "";
    optionFeedback[sel.field.id] = `Added “${res.value}”.`;
  } else if (res.kind === "exists") {
    optionInput[sel.field.id] = "";
    optionFeedback[sel.field.id] = `“${res.value}” is already in the list.`;
  } else {
    optionFeedback[sel.field.id] = res.message;
  }
}

function removeOption(sel: FieldSelection, option: string) {
  sel.options = sel.options.filter((o) => o !== option);
  optionFeedback[sel.field.id] = `Removed “${option}”.`;
}

function onSave() {
  const problem = draftProblem(name.value, description.value, selections.value);
  if (problem) {
    localError.value = problem;
    return;
  }
  localError.value = null;
  emit(
    "save",
    toDraft({ name: name.value, description: description.value, selections: selections.value, identifierId: identifierId.value }),
  );
}

const shownError = computed(() => localError.value ?? props.error ?? null);

const inputClass =
  "mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400";
const cardClass = "rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800";
const primaryBtn =
  "inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white";
const secondaryBtn =
  "inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800";
const smallBtn =
  "inline-flex h-8 items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-xs font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800";
</script>

<template>
  <form class="space-y-4" data-testid="category-builder" @submit.prevent="onSave">
    <!-- Name and description -->
    <section :class="cardClass" class="space-y-3">
      <div>
        <label :for="ids.name" class="block text-sm font-medium text-slate-900 dark:text-slate-100">Name</label>
        <input
          :id="ids.name"
          v-model="name"
          type="text"
          :maxlength="CATEGORY_NAME_MAX"
          placeholder="e.g. Sneakers"
          :class="inputClass"
          data-testid="builder-name"
        >
        <p v-if="nameSuggestion" class="mt-1 text-xs text-slate-600 dark:text-slate-400">
          Names usually look like
          <button
            type="button"
            class="font-medium underline"
            data-testid="builder-name-suggestion"
            @click="name = nameSuggestion"
          >
            {{ nameSuggestion }}</button>.
        </p>
      </div>
      <div>
        <label :for="ids.description" class="block text-sm font-medium text-slate-900 dark:text-slate-100">
          Description <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
        </label>
        <textarea
          :id="ids.description"
          v-model="description"
          rows="2"
          :maxlength="CATEGORY_DESCRIPTION_MAX"
          :class="inputClass"
          data-testid="builder-description"
        />
      </div>
    </section>

    <!-- Chosen fields -->
    <section :class="cardClass" class="space-y-3" aria-labelledby="builder-fields-heading">
      <div>
        <h2 id="builder-fields-heading" class="text-sm font-semibold text-slate-900 dark:text-slate-100">
          What you track for each item
        </h2>
        <p class="text-xs text-slate-600 dark:text-slate-400">
          Every item has a name, quantity, and condition. Add the details that matter for this kind of thing.
        </p>
      </div>
      <p
        v-if="selections.length === 0"
        class="rounded-lg bg-slate-50 p-3 text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
        data-testid="builder-no-fields"
      >
        No fields yet. Pick some from the list below.
      </p>
      <ul v-else class="space-y-3">
        <li
          v-for="sel in selections"
          :key="sel.field.id"
          class="rounded-lg border border-slate-200 p-3 dark:border-slate-700"
          :data-testid="`builder-selected-${sel.field.key}`"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="text-sm font-medium text-slate-900 dark:text-slate-100">
                {{ sel.field.title }}
                <span class="ml-1 text-xs font-normal text-slate-500 dark:text-slate-400">{{ typeLabel(sel.field) }}</span>
              </p>
              <p v-if="sel.field.sources.length > 0" class="text-xs text-slate-500 dark:text-slate-400">
                Used in {{ sourceNames(sel.field) }}
              </p>
            </div>
            <button
              type="button"
              :class="smallBtn"
              :aria-label="`Remove ${sel.field.title}`"
              :data-testid="`builder-remove-${sel.field.key}`"
              @click="removeField(sel.field.id)"
            >
              Remove
            </button>
          </div>

          <div v-if="sel.field.type === 'string'" class="mt-2 space-y-2">
            <p class="text-xs text-slate-600 dark:text-slate-400">
              <template v-if="sel.options.length === 0">
                People type anything here. Add options to make it a pick list.
              </template>
              <template v-else>Options to pick from:</template>
            </p>
            <ul v-if="sel.options.length > 0" class="flex flex-wrap gap-1.5" :aria-label="`${sel.field.title} options`">
              <li
                v-for="o in sel.options"
                :key="o"
                class="inline-flex items-center gap-1 rounded-full bg-slate-100 py-0.5 pl-2.5 pr-1 text-xs text-slate-800 dark:bg-slate-800 dark:text-slate-100"
              >
                {{ o }}
                <button
                  type="button"
                  class="rounded-full p-0.5 text-slate-500 hover:bg-slate-200 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-700 dark:hover:text-slate-100"
                  :aria-label="`Remove option ${o}`"
                  @click="removeOption(sel, o)"
                >
                  <Icon name="lucide:x" class="h-3 w-3" aria-hidden="true" />
                </button>
              </li>
            </ul>
            <div class="flex gap-2">
              <label class="min-w-0 flex-1">
                <span class="sr-only">New option for {{ sel.field.title }}</span>
                <input
                  v-model="optionInput[sel.field.id]"
                  type="text"
                  :placeholder="`Add an option for ${sel.field.title}`"
                  class="block w-full rounded-md border-slate-300 bg-white px-3 py-1.5 text-sm shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
                  :data-testid="`builder-option-input-${sel.field.key}`"
                  @keydown.enter.prevent="commitOption(sel, typedOption(sel.field.id))"
                >
              </label>
              <button
                type="button"
                :class="smallBtn"
                class="h-auto"
                :data-testid="`builder-option-add-${sel.field.key}`"
                @click="commitOption(sel, typedOption(sel.field.id))"
              >
                Add
              </button>
            </div>
            <p v-if="optionSuggestion(sel.field.id)" class="text-xs text-slate-600 dark:text-slate-400">
              Add it as
              <button
                type="button"
                class="font-medium underline"
                :data-testid="`builder-option-suggestion-${sel.field.key}`"
                @click="commitOption(sel, optionSuggestion(sel.field.id) ?? '')"
              >
                {{ optionSuggestion(sel.field.id) }}</button>?
              Keep your own capitals for names like “LEGO”.
            </p>
            <p
              v-if="optionFeedback[sel.field.id]"
              class="text-xs text-slate-600 dark:text-slate-400"
              aria-live="polite"
              :data-testid="`builder-option-feedback-${sel.field.key}`"
            >
              {{ optionFeedback[sel.field.id] }}
            </p>
          </div>
        </li>
      </ul>

      <div v-if="selections.length > 0">
        <label :for="ids.identifier" class="block text-xs font-medium text-slate-700 dark:text-slate-300">
          Which detail tells two items apart? <span class="font-normal text-slate-500 dark:text-slate-400">(optional)</span>
        </label>
        <select :id="ids.identifier" v-model="identifierId" :class="inputClass" data-testid="builder-identifier">
          <option value="">Just the name</option>
          <option v-for="sel in selections" :key="sel.field.id" :value="sel.field.id">{{ sel.field.title }}</option>
        </select>
      </div>
    </section>

    <!-- Field library -->
    <section :class="cardClass" class="space-y-3" aria-labelledby="builder-library-heading">
      <div>
        <h2 id="builder-library-heading" class="text-sm font-semibold text-slate-900 dark:text-slate-100">
          Fields other categories use
        </h2>
        <p class="text-xs text-slate-600 dark:text-slate-400">Pick any of them; you can add your own options after.</p>
      </div>
      <label :for="ids.search" class="sr-only">Search fields</label>
      <input
        :id="ids.search"
        v-model="search"
        type="search"
        placeholder="Search fields, e.g. year or theme"
        :class="inputClass"
        data-testid="builder-field-search"
      >
      <p v-if="libraryLoading && library.length === 0" class="text-sm text-slate-500 dark:text-slate-400">Loading fields…</p>
      <p v-else-if="groups.length === 0" class="text-sm text-slate-600 dark:text-slate-400" data-testid="builder-no-matches">
        No fields match “{{ search }}”.
      </p>
      <div v-for="g in groups" :key="g.source.slug" class="space-y-1" :data-testid="`builder-group-${g.source.slug}`">
        <h3 class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">From {{ g.source.name }}</h3>
        <ul class="divide-y divide-slate-100 dark:divide-slate-800">
          <li v-for="f in g.fields" :key="f.id" class="flex items-center justify-between gap-3 py-2">
            <div class="min-w-0">
              <p class="text-sm text-slate-900 dark:text-slate-100">
                {{ f.title }}
                <span class="ml-1 text-xs text-slate-500 dark:text-slate-400">{{ typeLabel(f) }}</span>
              </p>
              <p v-if="f.description" class="truncate text-xs text-slate-500 dark:text-slate-400">{{ f.description }}</p>
            </div>
            <button
              type="button"
              :class="smallBtn"
              :disabled="selectedIds.has(f.id)"
              :aria-label="selectedIds.has(f.id) ? `${f.title} added` : `Add ${f.title} (from ${g.source.name})`"
              :data-testid="`builder-add-${g.source.slug}-${f.key}`"
              @click="addField(f, g.source.slug)"
            >
              {{ selectedIds.has(f.id) ? "Added" : "Add" }}
            </button>
          </li>
        </ul>
      </div>
    </section>

    <!-- Preview -->
    <section v-if="selections.length > 0" :class="cardClass" class="space-y-2" data-testid="builder-preview">
      <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">Preview: adding an item</h2>
      <AttributeFields :schema="schema" :model-value="previewValues" @update:model-value="previewValues = $event" />
    </section>

    <div v-if="shownError" role="alert" class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100" data-testid="builder-error">
      {{ shownError }}
      <NuxtLink
        v-if="existing"
        :to="`/categories/${existing.slug}`"
        class="ml-1 font-medium underline"
        data-testid="builder-existing-link"
      >
        Open {{ existing.name }}
      </NuxtLink>
    </div>

    <div class="flex gap-2">
      <button type="submit" :class="primaryBtn" :disabled="busy" data-testid="builder-save">
        {{ submitLabel ?? "Save category" }}
      </button>
      <button type="button" :class="secondaryBtn" @click="emit('cancel')">Cancel</button>
    </div>
  </form>
</template>
