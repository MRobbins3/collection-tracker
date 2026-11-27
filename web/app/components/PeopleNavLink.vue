<script setup lang="ts">
import { onMounted, watch } from "vue";
import { requestCountLabel } from "~/utils/social";

/**
 * Header link to the People page, with a badge for requests waiting on you.
 * The count starts at 0 on the server and the client (hydration parity) and
 * loads in the browser once signed in, then again on every navigation.
 */
const { isSignedIn } = useAuth();
const { count, refreshCount } = useRequestCount();
const route = useRoute();

onMounted(() => {
  watch([isSignedIn, () => route.fullPath], () => void refreshCount(), { immediate: true });
});
</script>

<template>
  <NuxtLink
    to="/people"
    class="relative inline-flex items-center gap-1 text-sm text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-slate-50"
    data-testid="nav-people"
  >
    <Icon name="lucide:users" class="h-5 w-5 sm:hidden" aria-hidden="true" />
    <span class="sr-only sm:not-sr-only">People</span>
    <span
      v-if="count > 0"
      class="absolute -right-2 -top-1.5 min-w-[1.1rem] rounded-full bg-rose-600 px-1 text-center text-[0.65rem] font-semibold leading-4 text-white sm:static sm:ml-0.5"
      data-testid="nav-people-badge"
    ><span aria-hidden="true">{{ count }}</span><span class="sr-only">, {{ requestCountLabel(count) }}</span></span>
  </NuxtLink>
</template>
