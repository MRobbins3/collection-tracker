<script setup lang="ts">
import { computed } from "vue";

const { preference, cycle } = useTheme();

const label = computed(() => {
  switch (preference.value) {
    case "light":
      return "Light mode";
    case "dark":
      return "Dark mode";
    default:
      return "System theme";
  }
});

// Lucide via @nuxt/icon. Three genuinely different glyphs:
//   system → a monitor (follows the OS)
//   light  → a sun
//   dark   → a moon
const iconName = computed(() => {
  switch (preference.value) {
    case "light":
      return "lucide:sun";
    case "dark":
      return "lucide:moon";
    default:
      return "lucide:monitor";
  }
});
</script>

<template>
  <button
    type="button"
    class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-300 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
    :aria-label="`Change theme (current: ${label})`"
    :title="label"
    data-testid="theme-toggle"
    @click="cycle"
  >
    <Icon :name="iconName" class="h-5 w-5" aria-hidden="true" />
  </button>
</template>
