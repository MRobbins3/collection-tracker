<script setup lang="ts">
import { computed } from "vue";
import { avatarClass, initials } from "~/utils/avatar";

/**
 * Initials avatar in the user's stable palette color. Decorative by default
 * (the name is always shown or announced next to it), so it's hidden from
 * screen readers.
 */
const props = withDefaults(
  defineProps<{ name: string; color?: string; size?: "sm" | "md" | "lg" }>(),
  { color: "slate", size: "md" },
);

const sizeClass = computed(
  () => ({ sm: "h-8 w-8 text-xs", md: "h-10 w-10 text-sm", lg: "h-16 w-16 text-xl" })[props.size],
);
</script>

<template>
  <span
    class="inline-flex shrink-0 select-none items-center justify-center rounded-full font-semibold ring-2 ring-white dark:ring-slate-900"
    :class="[sizeClass, avatarClass(color)]"
    aria-hidden="true"
    data-testid="user-avatar"
  >{{ initials(name) }}</span>
</template>
