<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, useId } from "vue";
import type { DevPersona } from "~/types/api";

/**
 * Local/CI-only "Sign in as a test user" picker (ADR 0008). Rendered only
 * when GET /auth/providers says dev auth is on, which the API never allows
 * in production. A disclosure: the button toggles a panel listing the
 * seeded personas by display name; Escape or clicking outside closes it.
 */
const props = withDefaults(defineProps<{ variant?: "compact" | "full" }>(), { variant: "full" });

const { listPersonas, devLogin } = useAuth();

const open = ref(false);
const loading = ref(false);
const error = ref<string | null>(null);
const personas = ref<DevPersona[] | null>(null);
const signingIn = ref<string | null>(null);

const root = ref<HTMLElement | null>(null);
const trigger = ref<HTMLButtonElement | null>(null);
const panelId = useId();
const headingId = useId();

async function load() {
  loading.value = true;
  error.value = null;
  try {
    personas.value = await listPersonas();
  } catch {
    error.value = "Couldn’t load the test users. Is the API running?";
  } finally {
    loading.value = false;
  }
  await nextTick();
  root.value?.querySelector<HTMLButtonElement>("[data-persona]")?.focus();
}

async function toggle() {
  if (open.value) {
    close();
    return;
  }
  open.value = true;
  await load();
}

function close(returnFocus = false) {
  open.value = false;
  error.value = null;
  if (returnFocus) trigger.value?.focus();
}

async function choose(p: DevPersona) {
  signingIn.value = p.handle;
  error.value = null;
  try {
    await devLogin(p.handle);
    close();
  } catch {
    error.value = `Couldn’t sign in as ${p.display_name}. Try again.`;
  } finally {
    signingIn.value = null;
  }
}

function summary(p: DevPersona): string {
  if (p.collection_count === 0) return "No collections yet";
  const c = `${p.collection_count} collection${p.collection_count === 1 ? "" : "s"}`;
  const i = `${p.item_count} item${p.item_count === 1 ? "" : "s"}`;
  return `${c} · ${i}`;
}

function onPointerDown(e: PointerEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) close();
}
onMounted(() => document.addEventListener("pointerdown", onPointerDown));
onBeforeUnmount(() => document.removeEventListener("pointerdown", onPointerDown));
</script>

<template>
  <div ref="root" :class="props.variant === 'compact' ? 'relative' : 'w-full'" @keydown.esc="close(true)">
    <button
      ref="trigger"
      type="button"
      :aria-expanded="open ? 'true' : 'false'"
      :aria-controls="panelId"
      :class="
        props.variant === 'compact'
          ? 'inline-flex h-9 items-center rounded-md border border-amber-400 bg-amber-50 px-3 text-sm font-medium text-amber-900 hover:bg-amber-100 dark:border-amber-500/60 dark:bg-amber-950 dark:text-amber-100 dark:hover:bg-amber-900'
          : 'inline-flex h-10 items-center justify-center rounded-lg border border-amber-400 bg-amber-50 px-4 text-sm font-medium text-amber-900 hover:bg-amber-100 dark:border-amber-500/60 dark:bg-amber-950 dark:text-amber-100 dark:hover:bg-amber-900'
      "
      data-testid="dev-signin-button"
      @click="toggle"
    >
      {{ props.variant === "compact" ? "Test sign-in" : "Sign in as a test user" }}
    </button>

    <div
      v-if="open"
      :id="panelId"
      role="region"
      :aria-labelledby="headingId"
      :class="[
        'rounded-xl bg-white p-3 text-left shadow-lg ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-700',
        props.variant === 'compact' ? 'absolute right-0 top-full z-20 mt-2 w-72 max-w-[calc(100vw-2rem)]' : 'mt-3',
      ]"
      data-testid="dev-persona-picker"
    >
      <p :id="headingId" class="text-sm font-semibold text-slate-900 dark:text-slate-50">
        Sign in as a test user
      </p>
      <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
        For local testing only. These are made-up accounts, and this option is never available on the live site.
      </p>

      <p
        v-if="loading"
        class="mt-3 text-sm text-slate-500 dark:text-slate-400"
        role="status"
      >
        Loading test users…
      </p>
      <p
        v-else-if="personas && personas.length === 0"
        class="mt-3 text-sm text-slate-600 dark:text-slate-300"
        data-testid="dev-persona-empty"
      >
        No test users are set up yet. Restart the API with dev seed data turned on.
      </p>
      <ul v-else-if="personas" class="mt-2 space-y-1">
        <li v-for="p in personas" :key="p.handle">
          <button
            type="button"
            data-persona
            :disabled="signingIn !== null"
            class="flex w-full flex-col items-start rounded-lg px-3 py-2 text-left hover:bg-slate-100 focus:bg-slate-100 focus:outline-none disabled:opacity-60 dark:hover:bg-slate-800 dark:focus:bg-slate-800"
            :data-testid="`dev-persona-${p.handle}`"
            @click="choose(p)"
          >
            <span class="text-sm font-medium text-slate-900 dark:text-slate-50">
              {{ p.display_name }}
              <span v-if="signingIn === p.handle" class="font-normal text-slate-500 dark:text-slate-400">
                — signing in…
              </span>
            </span>
            <span class="text-xs text-slate-500 dark:text-slate-400">{{ summary(p) }}</span>
          </button>
        </li>
      </ul>

      <p
        v-if="error"
        class="mt-3 text-sm text-red-700 dark:text-red-400"
        role="alert"
        data-testid="dev-persona-error"
      >
        {{ error }}
      </p>
    </div>
  </div>
</template>
