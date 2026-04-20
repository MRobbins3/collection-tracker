<template>
  <div class="min-h-screen flex flex-col">
    <header class="sticky top-0 z-10 border-b border-slate-200 bg-white/90 backdrop-blur">
      <div class="mx-auto flex max-w-screen-sm items-center gap-3 px-4 py-3">
        <NuxtLink to="/" class="text-lg font-semibold tracking-tight text-slate-900">
          Collection Tracker
        </NuxtLink>
      </div>
    </header>
    <main class="mx-auto w-full max-w-screen-sm flex-1 px-4 py-4">
      <slot />
    </main>
    <footer class="mx-auto w-full max-w-screen-sm px-4 py-6 text-center text-xs text-slate-500">
      Browse anonymously. Sign in to start saving your collection.
    </footer>
  </div>
</template>
