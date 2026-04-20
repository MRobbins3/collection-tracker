<script setup lang="ts">
const route = useRoute();
const { isSignedIn, user } = useAuth();

useHead({ title: "Collection Tracker" });

const justSignedIn = computed(() => route.query.auth === "ok");
</script>

<template>
  <section class="space-y-4">
    <div
      v-if="justSignedIn && isSignedIn"
      class="rounded-lg bg-emerald-50 p-3 text-sm text-emerald-900"
      data-testid="signin-toast"
    >
      Welcome{{ user?.display_name ? `, ${user.display_name}` : "" }}! You’re signed in.
    </div>

    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900">Track anything you collect.</h1>
      <p class="mt-2 text-slate-600">
        Lego sets, Funko Pops, coins, stamps, plants — one app for all of it. Browse categories
        anonymously, or sign in to save what you own.
      </p>
    </div>

    <div class="flex flex-wrap gap-3">
      <NuxtLink
        to="/categories"
        class="inline-flex h-11 items-center justify-center rounded-lg bg-slate-900 px-5 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800 active:bg-slate-950"
      >
        Browse categories
      </NuxtLink>
    </div>
  </section>
</template>
