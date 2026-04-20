<script setup lang="ts">
const config = useRuntimeConfig();
</script>

<template>
  <div class="app">
    <header>
      <h1>Collection Tracker</h1>
      <p>Mobile-first collection tracking. Scaffold is live.</p>
    </header>
    <main>
      <p>API base: {{ config.public.apiBase }}</p>
    </main>
  </div>
</template>

<style>
.app {
  font-family: system-ui, -apple-system, sans-serif;
  max-width: 640px;
  margin: 0 auto;
  padding: 1rem;
}
</style>
