<script setup lang="ts">
import { computed } from "vue";
import DevSignIn from "./DevSignIn.vue";

defineProps<{ message?: string }>();
const { providers, loginURL } = useAuth();

// Same rules as AuthMenu: null providers (SSR, first client render) means
// the Google-only default.
const showDev = computed(() => providers.value?.dev === true);
const showGoogle = computed(() => !providers.value || providers.value.google || !providers.value.dev);
</script>

<template>
  <div
    class="rounded-xl bg-white p-6 text-center shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="signin-prompt"
  >
    <p class="text-sm text-slate-700 dark:text-slate-300">
      {{ message ?? "Sign in to start tracking your collections." }}
    </p>
    <div class="mt-4 flex flex-col items-center gap-2">
      <a
        v-if="showGoogle"
        :href="loginURL()"
        class="inline-flex h-10 items-center justify-center rounded-lg bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
        data-testid="signin-prompt-google"
      >
        Sign in with Google
      </a>
      <DevSignIn v-if="showDev" variant="full" />
    </div>
  </div>
</template>
