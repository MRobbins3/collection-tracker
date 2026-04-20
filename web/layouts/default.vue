<script setup lang="ts">
const { isSignedIn } = useAuth();
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header
      class="sticky top-0 z-10 border-b border-slate-200 bg-white/90 backdrop-blur dark:border-slate-800 dark:bg-slate-950/90"
    >
      <div class="mx-auto flex max-w-screen-sm items-center justify-between gap-3 px-4 py-3">
        <NuxtLink
          to="/"
          class="text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-50"
        >
          Collection Tracker
        </NuxtLink>
        <div class="flex items-center gap-3">
          <NuxtLink
            v-if="isSignedIn"
            to="/my"
            class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-slate-50"
            data-testid="nav-my-collections"
          >
            My collections
          </NuxtLink>
          <!--
            Wrapped in ClientOnly: the toggle's icon depends on a preference that
            only exists in the browser. The inline head script applies the
            correct `dark` class before first paint; rendering the icon is a
            post-hydration concern to avoid SSR/client disagreement.
          -->
          <ClientOnly>
            <ThemeToggle />
            <template #fallback>
              <div class="h-9 w-9" aria-hidden="true" />
            </template>
          </ClientOnly>
          <AuthMenu />
        </div>
      </div>
    </header>
    <main class="mx-auto w-full max-w-screen-sm flex-1 px-4 py-4">
      <slot />
    </main>
    <footer
      class="mx-auto w-full max-w-screen-sm px-4 py-6 text-center text-xs text-slate-500 dark:text-slate-500"
    >
      Browse anonymously. Sign in to start saving your collection.
    </footer>
  </div>
</template>
