<script setup lang="ts">
import type { CompareBucket } from "~/types/api";
import { parseBucket } from "~/utils/compare";

const route = useRoute();
const router = useRouter();
const handle = computed(() => String(route.params.handle));
const { isSignedIn, ready } = useAuth();

// The URL holds the state: ?category=<slug>&tab=<bucket>. Changing the
// category or tab replaces the entry, so Back leaves the page.
const category = computed<string | undefined>({
  get: () => (route.query.category ? String(route.query.category) : undefined),
  set: (slug) => void router.replace({ query: { ...route.query, category: slug, tab: undefined } }),
});
const bucket = computed<CompareBucket>({
  get: () => parseBucket(route.query.tab),
  set: (b) => void router.replace({ query: { ...route.query, tab: b === "both" ? undefined : b } }),
});

useHead({ title: "Compare collections · Collection Tracker" });
</script>

<template>
  <div>
    <p v-if="!ready" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
    <SignInPrompt v-else-if="!isSignedIn" message="Sign in to compare your collection with someone else’s." />
    <CollectionCompare v-else v-model:category="category" v-model:bucket="bucket" :handle="handle" />
  </div>
</template>
