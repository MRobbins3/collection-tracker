<script setup lang="ts">
const route = useRoute();
const handle = String(route.params.handle);
const { ready } = useAuth();
const { data, pending, error, refresh } = usePublicProfile(handle);

useHead(() => ({
  title: data.value ? `${data.value.profile.display_name} · Collection Tracker` : "Profile · Collection Tracker",
}));
</script>

<template>
  <div>
    <p v-if="!ready || (pending && !data)" class="text-sm text-slate-500 dark:text-slate-400" data-testid="profile-loading">
      Loading profile…
    </p>
    <div
      v-else-if="error"
      role="alert"
      class="rounded-xl bg-rose-50 p-4 text-sm text-rose-800 dark:bg-rose-950 dark:text-rose-200"
    >
      We couldn’t load this profile.
      <button type="button" class="font-medium underline" @click="() => refresh()">Try again</button>
    </div>
    <PublicProfileView v-else-if="data" :data="data" />
    <section v-else class="space-y-3 rounded-xl bg-slate-100 p-6 text-center dark:bg-slate-800" data-testid="profile-not-found">
      <h1 class="text-lg font-semibold text-slate-900 dark:text-slate-50">Profile not found</h1>
      <p class="text-sm text-slate-600 dark:text-slate-300">
        There’s no profile here, or its owner keeps it private.
      </p>
      <NuxtLink to="/" class="inline-block text-sm text-slate-700 underline underline-offset-2 dark:text-slate-200">
        Browse categories
      </NuxtLink>
    </section>
  </div>
</template>
