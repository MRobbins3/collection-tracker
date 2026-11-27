<script setup lang="ts">
import { computed } from "vue";
import type { AttributeSchema } from "~/types/api";

const route = useRoute();
const handle = String(route.params.handle);
const id = String(route.params.id);
const { ready, isSignedIn } = useAuth();
const { data, pending, error, refresh } = usePublicCollection(handle, id);

// The collection's category comes with it (ADR 0013): its schema gives
// attribute fields their human labels, even for a category its owner built.
const schema = computed(() => data.value?.category?.attribute_schema as AttributeSchema | undefined);

useHead(() => ({
  title: data.value
    ? `${data.value.collection.name} · ${data.value.owner.display_name} · Collection Tracker`
    : "Collection · Collection Tracker",
}));
</script>

<template>
  <div>
    <p v-if="!ready || (pending && !data)" class="text-sm text-slate-500 dark:text-slate-400">Loading collection…</p>
    <div
      v-else-if="error"
      role="alert"
      class="rounded-xl bg-rose-50 p-4 text-sm text-rose-800 dark:bg-rose-950 dark:text-rose-200"
    >
      We couldn’t load this collection.
      <button type="button" class="font-medium underline" @click="() => refresh()">Try again</button>
    </div>
    <section v-else-if="data" class="space-y-4">
      <NuxtLink
        :to="`/u/${data.owner.handle}`"
        class="inline-flex items-center gap-2 text-sm text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-slate-50"
        data-testid="collection-owner-link"
      >
        <UserAvatar :name="data.owner.display_name" :color="data.owner.avatar_color" size="sm" />
        <span>{{ data.owner.display_name }}</span>
      </NuxtLink>
      <div>
        <h1 class="text-xl font-bold tracking-tight text-slate-900 dark:text-slate-50">{{ data.collection.name }}</h1>
        <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">
          {{ data.collection.category_name }} · {{ data.items.length }} item{{ data.items.length === 1 ? "" : "s" }}
        </p>
        <NuxtLink
          v-if="data.is_self"
          :to="`/my/${data.collection.id}`"
          class="mt-2 inline-block text-sm text-slate-700 underline underline-offset-2 dark:text-slate-300"
        >
          Edit in My collections
        </NuxtLink>
        <NuxtLink
          v-else-if="isSignedIn"
          :to="{ path: `/u/${data.owner.handle}/compare`, query: { category: data.category.slug } }"
          class="mt-2 inline-flex h-9 items-center gap-1.5 rounded-md border border-slate-300 px-3 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          data-testid="collection-compare-link"
        >
          <Icon name="lucide:git-compare" class="h-4 w-4" aria-hidden="true" />
          Compare with my {{ data.category.name }}
        </NuxtLink>
      </div>
      <ul v-if="data.items.length > 0" class="space-y-2" data-testid="public-items-list">
        <ItemCard v-for="item in data.items" :key="item.id" :item="item" :schema="schema" readonly />
      </ul>
      <p
        v-else
        class="rounded-xl bg-slate-100 p-6 text-center text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300"
      >
        Nothing in this collection yet.
      </p>
    </section>
    <section v-else class="space-y-3 rounded-xl bg-slate-100 p-6 text-center dark:bg-slate-800" data-testid="collection-not-found">
      <h1 class="text-lg font-semibold text-slate-900 dark:text-slate-50">Collection not found</h1>
      <p class="text-sm text-slate-600 dark:text-slate-300">
        It doesn’t exist, or its owner doesn’t share it with you.
      </p>
    </section>
  </div>
</template>
