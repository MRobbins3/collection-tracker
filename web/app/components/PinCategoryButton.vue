<script setup lang="ts">
/**
 * Add a global category to your collection overview (pin it), or remove one
 * you added. A category you already have collections in links to it in the
 * overview instead, since removing the pin wouldn't hide it.
 */
defineProps<{
  slug: string;
  name: string;
  state: "none" | "pinned" | "collecting";
  busy?: boolean;
}>();
const emit = defineEmits<{ add: []; remove: [] }>();
</script>

<template>
  <NuxtLink
    v-if="state === 'collecting'"
    :to="`/my?category=${slug}`"
    class="inline-flex h-9 shrink-0 items-center gap-1 rounded-lg px-2 text-sm font-medium text-emerald-700 hover:underline dark:text-emerald-300"
    :data-testid="`pin-category-${slug}`"
  >In my collection</NuxtLink>
  <span v-else-if="state === 'pinned'" class="inline-flex shrink-0 items-center gap-2">
    <span class="inline-flex items-center gap-1 text-sm font-medium text-emerald-700 dark:text-emerald-300">
      <Icon name="lucide:check" class="h-4 w-4" aria-hidden="true" />
      Added
    </span>
    <button
      type="button"
      :disabled="busy"
      :aria-label="`Remove ${name} from my collection`"
      class="inline-flex h-9 items-center rounded-lg border border-slate-300 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
      :data-testid="`unpin-category-${slug}`"
      @click="emit('remove')"
    >
      Remove
    </button>
  </span>
  <button
    v-else
    type="button"
    :disabled="busy"
    :aria-label="`Add ${name} to my collection`"
    class="inline-flex h-9 shrink-0 items-center gap-1 rounded-lg bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
    :data-testid="`pin-category-${slug}`"
    @click="emit('add')"
  ><Icon name="lucide:plus" class="h-4 w-4" aria-hidden="true" />Add</button>
</template>
