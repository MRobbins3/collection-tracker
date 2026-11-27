<script setup lang="ts">
useHead({ title: "My submissions · Collection Tracker" });

const { isSignedIn } = useAuth();
const { data, status, error, refresh } = useMySubmissions();
const loading = computed(() => status.value === "pending" || status.value === "idle");
</script>

<template>
  <section class="space-y-4">
    <header>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50">My submissions</h1>
      <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Catalog entries and corrections you suggested, and what the reviewers decided.
      </p>
    </header>

    <SignInPrompt v-if="!isSignedIn" message="Sign in to see your catalog suggestions." />

    <template v-else>
      <p v-if="loading && data.submissions.length === 0" class="text-sm text-slate-500 dark:text-slate-400">
        Loading…
      </p>
      <div
        v-else-if="error"
        role="alert"
        class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      >
        We couldn’t load your submissions.
        <button type="button" class="underline" @click="refresh()">Try again</button>
      </div>
      <div
        v-else-if="data.submissions.length === 0"
        class="rounded-xl bg-slate-100 p-4 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
        data-testid="submissions-empty"
      >
        You haven’t suggested anything yet. When something you own isn’t in a category’s catalog, use
        “Suggest this for the catalog” on the item.
      </div>
      <SubmissionList v-else :submissions="data.submissions" />
    </template>
  </section>
</template>
