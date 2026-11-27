<script setup lang="ts">
import { computed } from "vue";

/**
 * Renders its slot only for admins (ADR 0012). It's a courtesy, not the
 * guard: the API refuses non-admins on every admin route. Auth state loads
 * after hydration, so "Loading…" shows until it does (hydration parity).
 */
const { user, isSignedIn, ready } = useAuth();
const isAdmin = computed(() => user.value?.role === "admin");
</script>

<template>
  <p v-if="!ready" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
  <SignInPrompt v-else-if="!isSignedIn" message="Sign in to review catalog suggestions." />
  <div
    v-else-if="!isAdmin"
    class="rounded-lg bg-slate-100 p-4 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200"
    data-testid="admin-only"
  >
    This page is for catalog reviewers.
  </div>
  <slot v-else />
</template>
