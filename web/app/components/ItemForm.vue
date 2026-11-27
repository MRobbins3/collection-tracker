<script setup lang="ts">
import { computed, ref, reactive } from "vue";
import AttributeFields from "./AttributeFields.vue";
import type { AttributeSchema, Item } from "~/types/api";
import { collapseWhitespace, suggestTitleCase } from "~/utils/text";

const props = defineProps<{
  schema?: AttributeSchema;
  initial?: Partial<Item>;
  submitLabel?: string;
}>();

const emit = defineEmits<{
  (e: "submit", payload: { name: string; quantity: number; condition?: string; attributes: Record<string, unknown> }): void;
  (e: "cancel"): void;
}>();

const name = ref<string>(props.initial?.name ?? "");
const quantity = ref<number>(props.initial?.quantity ?? 1);
const condition = ref<string>(props.initial?.condition ?? "");
const attrs = reactive<Record<string, unknown>>({ ...(props.initial?.attributes ?? {}) });

const submitting = ref(false);
const error = ref<string | null>(null);

const nameSuggestion = computed(() => suggestTitleCase(name.value));

/** Text attributes get the same whitespace cleanup the API applies; blanks are dropped. */
function cleanedAttributes(): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(attrs)) {
    const value = typeof v === "string" ? collapseWhitespace(v) : v;
    if (value !== undefined && value !== "") out[k] = value;
  }
  return out;
}

async function onSubmit() {
  error.value = null;
  const trimmed = collapseWhitespace(name.value);
  if (trimmed.length < 1 || trimmed.length > 200) {
    error.value = "Name must be 1–200 characters.";
    return;
  }
  if (quantity.value < 1 || quantity.value > 1_000_000) {
    error.value = "Quantity must be at least 1.";
    return;
  }
  submitting.value = true;
  try {
    emit("submit", {
      name: trimmed,
      quantity: quantity.value,
      condition: collapseWhitespace(condition.value) || undefined,
      attributes: cleanedAttributes(),
    });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <form class="space-y-3" data-testid="item-form" @submit.prevent="onSubmit">
    <div>
      <label class="block">
        <span class="block text-xs font-medium text-slate-700 dark:text-slate-300">Name</span>
        <input
          v-model="name"
          type="text"
          maxlength="200"
          placeholder="e.g. Millennium Falcon UCS"
          class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
          data-testid="item-form-name"
        />
      </label>
      <p v-if="nameSuggestion" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
        Tip: names usually look best in Title Case.
        <button
          type="button"
          class="font-medium text-slate-900 underline underline-offset-2 dark:text-slate-100"
          data-testid="item-form-title-case"
          @click="name = nameSuggestion ?? name"
        >
          Use “{{ nameSuggestion }}”
        </button>
      </p>
    </div>

    <div class="grid grid-cols-2 gap-3">
      <label class="block">
        <span class="block text-xs font-medium text-slate-700 dark:text-slate-300">Quantity</span>
        <input
          v-model.number="quantity"
          type="number"
          min="1"
          class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:focus:border-slate-400 dark:focus:ring-slate-400"
          data-testid="item-form-quantity"
        />
      </label>
      <label class="block">
        <span class="block text-xs font-medium text-slate-700 dark:text-slate-300">Condition / variant</span>
        <input
          v-model="condition"
          type="text"
          maxlength="200"
          placeholder="New, sealed"
          class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
          data-testid="item-form-condition"
        />
      </label>
    </div>

    <AttributeFields
      v-if="schema"
      :schema="schema"
      :model-value="attrs"
      @update:model-value="Object.assign(attrs, $event)"
    />

    <p v-if="error" class="text-xs text-rose-700 dark:text-rose-300" data-testid="item-form-error">
      {{ error }}
    </p>

    <div class="flex gap-2">
      <button
        type="submit"
        :disabled="submitting"
        class="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        data-testid="item-form-submit"
      >
        {{ submitLabel ?? "Save" }}
      </button>
      <button
        type="button"
        class="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
        data-testid="item-form-cancel"
        @click="emit('cancel')"
      >
        Cancel
      </button>
    </div>
  </form>
</template>
