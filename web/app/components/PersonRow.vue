<script setup lang="ts">
import UserAvatar from "./UserAvatar.vue";
import type { Person } from "~/types/api";

/**
 * One person in a friends, followers, requests, or search list: avatar,
 * name, and handle linking to their profile, plus an actions slot.
 */
defineProps<{ person: Person }>();
</script>

<template>
  <li
    class="flex flex-wrap items-center gap-3 rounded-xl bg-white p-3 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    data-testid="person-row"
  >
    <NuxtLink :to="`/u/${person.handle}`" class="flex min-w-0 flex-1 items-center gap-3">
      <UserAvatar :name="person.display_name" :color="person.avatar_color" />
      <span class="min-w-0">
        <span class="block truncate text-sm font-medium text-slate-900 dark:text-slate-50">{{ person.display_name }}</span>
        <span class="block truncate text-xs text-slate-500 dark:text-slate-400">@{{ person.handle }}</span>
      </span>
    </NuxtLink>
    <div v-if="$slots.default" class="flex flex-wrap items-center gap-2">
      <slot />
    </div>
  </li>
</template>
