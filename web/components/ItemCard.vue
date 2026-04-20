<script setup lang="ts">
import { computed, ref } from "vue";
import ItemForm from "./ItemForm.vue";
import type { AttributeSchema, Item } from "~/types/api";

const props = defineProps<{ item: Item; schema?: AttributeSchema }>();

const emit = defineEmits<{
  (e: "update", input: { name: string; quantity: number; condition?: string; attributes: Record<string, unknown> }): void;
  (e: "delete"): void;
}>();

const editing = ref(false);

// Present a friendly `Title: value` line per attribute that the user filled in.
// Keys that have no title fall back to a humanized version of the raw key so
// old data doesn't ever show `set_number:`.
const visibleAttributes = computed<{ title: string; value: string }[]>(() => {
  const props_ = props.schema?.properties ?? {};
  const filled = Object.entries(props.item.attributes).filter(([, v]) => v !== undefined && v !== null && v !== "");
  return filled.map(([key, value]) => ({
    title: props_[key]?.title ?? humanize(key),
    value: String(value),
  }));
});

function humanize(key: string): string {
  const spaced = key.replace(/_/g, " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}
</script>

<template>
  <li
    class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200"
    :data-testid="`item-${item.id}`"
  >
    <div v-if="!editing">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <h3 class="truncate text-base font-semibold text-slate-900">{{ item.name }}</h3>
          <p class="mt-0.5 text-xs text-slate-500">
            Qty {{ item.quantity }}<span v-if="item.condition"> · {{ item.condition }}</span>
          </p>
          <dl v-if="visibleAttributes.length > 0" class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
            <template v-for="a in visibleAttributes" :key="a.title">
              <dt class="text-slate-500">{{ a.title }}</dt>
              <dd class="text-slate-900">{{ a.value }}</dd>
            </template>
          </dl>
        </div>
        <div class="flex shrink-0 gap-1">
          <button
            type="button"
            class="rounded-md border border-slate-200 px-2 py-1 text-xs text-slate-700 hover:bg-slate-50"
            :data-testid="`item-${item.id}-edit`"
            @click="editing = true"
          >
            Edit
          </button>
          <button
            type="button"
            class="rounded-md border border-rose-200 px-2 py-1 text-xs text-rose-700 hover:bg-rose-50"
            :data-testid="`item-${item.id}-delete`"
            @click="emit('delete')"
          >
            Delete
          </button>
        </div>
      </div>
    </div>
    <ItemForm
      v-else
      :schema="schema"
      :initial="item"
      submit-label="Save changes"
      @submit="
        (payload) => {
          editing = false;
          emit('update', payload);
        }
      "
      @cancel="editing = false"
    />
  </li>
</template>
