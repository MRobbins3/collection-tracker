<script setup lang="ts">
import RelationshipActions from "./RelationshipActions.vue";
import UserAvatar from "./UserAvatar.vue";
import type { PublicProfileResponse } from "~/types/api";
import { hiddenCollectionsMessage, ownVisibilityNote } from "~/utils/privacy";

/**
 * Someone's profile as the API allowed this viewer to see it: the card, the
 * friend/follow buttons for a signed-in visitor, and their collections
 * read-only when visible. No edit controls, even for the owner (they edit
 * from My collections and Settings). Emits "changed" when a friend or
 * follow action may have changed what the viewer can see.
 */
defineProps<{ data: PublicProfileResponse }>();
const emit = defineEmits<{ (e: "changed"): void }>();
</script>

<template>
  <section class="space-y-4">
    <div
      class="flex items-start gap-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
    >
      <UserAvatar :name="data.profile.display_name" :color="data.profile.avatar_color" size="lg" />
      <div class="min-w-0 flex-1">
        <h1 class="truncate text-xl font-bold tracking-tight text-slate-900 dark:text-slate-50">
          {{ data.profile.display_name }}
        </h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">@{{ data.profile.handle }}</p>
        <p v-if="data.profile.bio" class="mt-2 text-sm text-slate-700 dark:text-slate-300">
          {{ data.profile.bio }}
        </p>
        <RelationshipActions
          v-if="data.relationship"
          :handle="data.profile.handle"
          :name="data.profile.display_name"
          :relationship="data.relationship"
          @changed="emit('changed')"
        />
        <div v-if="data.is_self" class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1">
          <NuxtLink
            to="/settings"
            class="inline-flex h-9 items-center rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
            data-testid="profile-edit-link"
          >
            Edit profile
          </NuxtLink>
          <p class="text-xs text-slate-500 dark:text-slate-400" data-testid="profile-visibility-note">
            {{ ownVisibilityNote(data.collection_visibility) }}
          </p>
        </div>
      </div>
    </div>

    <h2 class="text-base font-semibold text-slate-900 dark:text-slate-50">Collections</h2>

    <p
      v-if="!data.collections_visible"
      class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      data-testid="profile-collections-hidden"
    >
      {{ hiddenCollectionsMessage(data.collection_visibility, data.profile.display_name) }}
    </p>
    <ul v-else-if="data.collections.length > 0" class="space-y-2" data-testid="profile-collections">
      <li
        v-for="c in data.collections"
        :key="c.id"
        class="rounded-xl bg-white shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
      >
        <NuxtLink :to="`/u/${data.profile.handle}/c/${c.id}`" class="block p-4">
          <h3 class="truncate text-base font-semibold text-slate-900 dark:text-slate-50">{{ c.name }}</h3>
          <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            {{ c.category_name }} · {{ c.item_count }} item{{ c.item_count === 1 ? "" : "s" }}
          </p>
        </NuxtLink>
      </li>
    </ul>
    <p
      v-else
      class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      data-testid="profile-collections-empty"
    >
      No collections yet.
    </p>
  </section>
</template>
