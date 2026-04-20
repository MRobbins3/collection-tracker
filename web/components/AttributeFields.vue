<script setup lang="ts">
import { computed } from "vue";
import type { AttributeSchema } from "~/types/api";

const props = defineProps<{
  schema?: AttributeSchema;
  modelValue: Record<string, unknown>;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: Record<string, unknown>): void;
}>();

interface FieldDef {
  key: string;
  title: string;
  description?: string;
  type: "string" | "integer" | "number" | "boolean";
}

const fields = computed<FieldDef[]>(() => {
  const props_ = props.schema?.properties ?? {};
  return Object.entries(props_).map(([key, def]) => ({
    key,
    title: def.title ?? humanize(key),
    description: def.description,
    type: def.type,
  }));
});

function humanize(key: string): string {
  const spaced = key.replace(/_/g, " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function onChange(key: string, value: unknown) {
  emit("update:modelValue", { ...props.modelValue, [key]: value });
}

function asString(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function asNumber(v: unknown): string {
  return typeof v === "number" && !Number.isNaN(v) ? String(v) : "";
}
</script>

<template>
  <div v-if="fields.length > 0" class="space-y-3" data-testid="attribute-fields">
    <label v-for="f in fields" :key="f.key" class="block">
      <span class="block text-xs font-medium text-slate-700 dark:text-slate-300">{{ f.title }}</span>
      <span
        v-if="f.description"
        class="block text-[11px] text-slate-500 dark:text-slate-400"
      >
        {{ f.description }}
      </span>

      <input
        v-if="f.type === 'string'"
        type="text"
        :value="asString(modelValue[f.key])"
        class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        :data-testid="`attribute-field-${f.key}`"
        @input="onChange(f.key, ($event.target as HTMLInputElement).value || undefined)"
      />
      <input
        v-else-if="f.type === 'integer' || f.type === 'number'"
        type="number"
        :value="asNumber(modelValue[f.key])"
        class="mt-1 block w-full rounded-md border-slate-300 bg-white px-3 py-2 text-base shadow-sm focus:border-slate-900 focus:outline-none focus:ring-1 focus:ring-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:placeholder-slate-500 dark:focus:border-slate-400 dark:focus:ring-slate-400"
        :data-testid="`attribute-field-${f.key}`"
        @input="
          onChange(
            f.key,
            ($event.target as HTMLInputElement).value === ''
              ? undefined
              : Number(($event.target as HTMLInputElement).value),
          )
        "
      />
    </label>
  </div>
</template>
