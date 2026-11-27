<script setup lang="ts">
import { computed } from "vue";
import type { AttributeSchema, Category } from "~/types/api";

const route = useRoute();
const handle = String(route.params.handle);
const id = String(route.params.id);
const { ready } = useAuth();
const api = useApi();
const { data, pending, error, refresh } = usePublicCollection(handle, id);

// The category's schema gives attribute fields their human labels.
const { data: category } = useAsyncData<Category | null>(
  `profile-collection-category:${handle}:${id}`,
  async () => (data.value ? api.get<Category>(`/categories/${data.value.collection.category_slug}`) : null),
  { server: false, watch: [data], default: () => null },
);
const schema = computed(() => category.value?.attribute_schema as AttributeSchema | undefined);

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
