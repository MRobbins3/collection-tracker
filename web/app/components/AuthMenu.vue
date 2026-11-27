<script setup lang="ts">
const { user, isSignedIn, loginURL, logout } = useAuth();
</script>

<template>
  <div v-if="isSignedIn" class="flex items-center gap-2" data-testid="auth-menu-signed-in">
    <span
      class="max-w-[10rem] truncate text-sm text-slate-600 dark:text-slate-300"
      :title="user?.email"
    >
      {{ user?.display_name }}
    </span>
    <button
      type="button"
      class="inline-flex h-9 items-center rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
      data-testid="logout-button"
      @click="logout"
    >
      Sign out
    </button>
  </div>
  <a
    v-else
    :href="loginURL()"
    class="inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
    data-testid="signin-link"
  >
    Sign in with Google
  </a>
</template>
