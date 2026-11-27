<script setup lang="ts">
import type { Profile } from "~/types/api";

const { isSignedIn, ready, user } = useAuth();
const { data: profile, pending, error, refresh } = useMyProfile();

useHead({ title: "Settings · Collection Tracker" });

function onSaved(p: Profile) {
  profile.value = p;
}

// The API has already cleared the session cookie. Leave the page first, then
// drop the signed-in state, so this page doesn't flash its sign-in prompt.
async function onDeleted() {
  await navigateTo("/");
  user.value = null;
}
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-xl font-bold tracking-tight text-slate-900 dark:text-slate-50">Settings</h1>
      <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
        Your profile, and who can see your collections.
      </p>
    </div>

    <SignInPrompt v-if="ready && !isSignedIn" message="Sign in to edit your profile and privacy settings." />

    <template v-else-if="isSignedIn">
      <p v-if="pending && !profile" class="text-sm text-slate-500 dark:text-slate-400">Loading your settings…</p>
      <div
        v-else-if="error"
        role="alert"
        class="rounded-xl bg-rose-50 p-4 text-sm text-rose-800 dark:bg-rose-950 dark:text-rose-200"
      >
        We couldn’t load your settings.
        <button type="button" class="font-medium underline" @click="() => refresh()">Try again</button>
      </div>
      <template v-else-if="profile">
        <ProfileForm :profile="profile" @saved="onSaved" />
        <p class="text-sm">
          <NuxtLink
            :to="`/u/${profile.handle}`"
            class="text-slate-700 underline underline-offset-2 dark:text-slate-300"
            data-testid="settings-view-profile"
          >
            See your profile as others do
          </NuxtLink>
        </p>
        <DeleteAccount :handle="profile.handle" @deleted="onDeleted" />
      </template>
    </template>

    <p v-else class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
  </section>
</template>
