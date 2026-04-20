<script setup lang="ts">
defineProps<{ message?: string }>();
const { loginURL } = useAuth();
</script>

<template>
  <div
    class="rounded-xl bg-white p-6 text-center shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="signin-prompt"
  >
    <p class="text-sm text-slate-700 dark:text-slate-300">
      {{ message ?? "Sign in to start tracking your collections." }}
    </p>
    <a
      :href="loginURL()"
      class="mt-4 inline-flex h-10 items-center justify-center rounded-lg bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
    >
      Sign in with Google
    </a>
  </div>
</template>
