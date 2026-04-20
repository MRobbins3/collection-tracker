<script setup lang="ts">
const { surface, promptInstall, dismiss } = useInstall();

async function onInstall() {
  const outcome = await promptInstall();
  // Chrome doesn't fire appinstalled on "dismissed", so bail manually.
  if (outcome === "dismissed") dismiss();
}
</script>

<template>
  <section
    v-if="surface !== 'hidden'"
    class="rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="install-prompt"
  >
    <div class="flex items-start gap-3">
      <div class="min-w-0 flex-1">
        <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
          Install Collection Tracker
        </h2>
        <p
          v-if="surface === 'android'"
          class="mt-1 text-sm text-slate-600 dark:text-slate-400"
          data-testid="install-prompt-android"
        >
          Add the app to your home screen for a full-screen, offline-capable experience.
        </p>
        <p
          v-else
          class="mt-1 text-sm text-slate-600 dark:text-slate-400"
          data-testid="install-prompt-ios"
        >
          Tap the <span class="font-medium">Share</span> button in Safari, then
          <span class="font-medium">Add to Home Screen</span>.
        </p>
      </div>
      <button
        type="button"
        class="shrink-0 text-xs text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
        data-testid="install-prompt-dismiss"
        @click="dismiss"
      >
        Dismiss
      </button>
    </div>
    <button
      v-if="surface === 'android'"
      type="button"
      class="mt-3 inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
      data-testid="install-prompt-install"
      @click="onInstall"
    >
      Install
    </button>
  </section>
</template>
