<script setup lang="ts">
import type { PeopleTab } from "~/types/api";

useHead({ title: "People · Collection Tracker" });

const { isSignedIn, ready } = useAuth();
const route = useRoute();
const router = useRouter();

const TABS: PeopleTab[] = ["friends", "followers", "following", "requests"];
const tab = computed<PeopleTab>({
  get: () => {
    const q = String(route.query.tab ?? "");
    return (TABS as string[]).includes(q) ? (q as PeopleTab) : "friends";
  },
  set: (t) => void router.replace({ query: { ...route.query, tab: t === "friends" ? undefined : t } }),
});
</script>

<template>
  <section class="space-y-4">
    <header>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 dark:text-slate-50">People</h1>
      <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Friends and followers can see collections you share with them (choose who in Settings).
      </p>
    </header>

    <p v-if="!ready" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
    <SignInPrompt v-else-if="!isSignedIn" message="Sign in to find people and see your friends and followers." />
    <template v-else>
      <PeopleSearch />
      <PeopleHub v-model:tab="tab" />
    </template>
  </section>
</template>
