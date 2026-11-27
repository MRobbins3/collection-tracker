<script setup lang="ts">
import { identifierTitle } from "~/utils/catalog";
import type { AttributeSchema, Category, ModerationRequest } from "~/types/api";

useHead({ title: "Review · Collection Tracker" });

const route = useRoute();
const id = route.params.id as string;
const { user } = useAuth();
const { get } = useModerationAdmin();
const api = useApi();

const request = ref<ModerationRequest | null>(null);
const category = ref<Category | null>(null);
const loading = ref(false);
const notFound = ref(false);
const failed = ref(false);

async function load() {
  loading.value = true;
  failed.value = false;
  notFound.value = false;
  try {
    request.value = await get(id);
    if (category.value?.slug !== request.value.category_slug) {
      category.value = await api.get<Category>(`/categories/${request.value.category_slug}`);
    }
  } catch (e) {
    const status = (e as { statusCode?: number })?.statusCode;
    if (status === 404) notFound.value = true;
    else {
      console.error(e);
      failed.value = true;
    }
  } finally {
    loading.value = false;
  }
}

// Load once we know the viewer is an admin (auth loads after hydration).
watch(
  () => user.value?.role,
  (role) => {
    if (role === "admin" && !request.value) void load();
  },
  { immediate: true },
);

const schema = computed(() => category.value?.attribute_schema as AttributeSchema | undefined);
const idTitle = computed(() => identifierTitle(category.value));
</script>

<template>
  <section class="space-y-4">
    <NuxtLink
      to="/admin/moderation"
      class="inline-flex items-center text-sm text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-100"
    >
      ← Moderation queue
    </NuxtLink>
    <AdminOnly>
      <p v-if="loading && !request" class="text-sm text-slate-500 dark:text-slate-400">Loading…</p>
      <div
        v-else-if="notFound"
        class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      >
        We couldn’t find this request.
      </div>
      <div
        v-else-if="failed"
        role="alert"
        class="rounded-lg bg-rose-50 p-3 text-sm text-rose-900 dark:bg-rose-950 dark:text-rose-100"
      >
        We couldn’t load this request.
        <button type="button" class="underline" @click="load()">Try again</button>
      </div>
      <ModerationReview
        v-else-if="request"
        :request="request"
        :schema="schema"
        :identifier-title="idTitle"
        @decided="(r) => (request = r)"
        @reload="load()"
      />
    </AdminOnly>
  </section>
</template>
