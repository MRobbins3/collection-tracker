<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId } from "vue";
import DevSignIn from "./DevSignIn.vue";
import UserAvatar from "./UserAvatar.vue";

const { user, isSignedIn, providers, loginURL, logout } = useAuth();

// providers is null during SSR and the first client render (it loads in
// onNuxtReady, after hydration), so null must render the Google-only default. Google is
// hidden only when we know it's off and test-user sign-in is the way in.
const showDev = computed(() => providers.value?.dev === true);
const showGoogle = computed(() => !providers.value || providers.value.google || !providers.value.dev);
// Admin navigation only; the API enforces the role on every admin route.
const isAdmin = computed(() => user.value?.role === "admin");

// Account menu: a disclosure (button + list of links), not an ARIA menu, so
// links stay links and Tab works as usual. Escape and clicking outside close it.
const open = ref(false);
const root = ref<HTMLElement | null>(null);
const trigger = ref<HTMLButtonElement | null>(null);
const menuId = useId();

async function toggle() {
  open.value = !open.value;
  if (open.value) {
    await nextTick();
    root.value?.querySelector<HTMLElement>("[data-menu-item]")?.focus();
  }
}

function close(returnFocus = false) {
  open.value = false;
  if (returnFocus) trigger.value?.focus();
}

async function onLogout() {
  close();
  await logout();
}

function onPointerDown(e: PointerEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) close();
}
onMounted(() => document.addEventListener("pointerdown", onPointerDown));
onBeforeUnmount(() => document.removeEventListener("pointerdown", onPointerDown));

const itemClass =
  "block w-full rounded-md px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-100 focus:bg-slate-100 focus:outline-none dark:text-slate-200 dark:hover:bg-slate-800 dark:focus:bg-slate-800";
</script>

<template>
  <div
    v-if="isSignedIn && user"
    ref="root"
    class="relative"
    data-testid="auth-menu-signed-in"
    @keydown.escape="close(true)"
  >
    <button
      ref="trigger"
      type="button"
      class="flex items-center gap-2 rounded-full p-0.5 pr-2 hover:bg-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-400 dark:hover:bg-slate-800"
      :aria-expanded="open ? 'true' : 'false'"
      aria-haspopup="true"
      :aria-controls="menuId"
      data-testid="account-menu-button"
      @click="toggle"
    >
      <UserAvatar :name="user.display_name" :color="user.avatar_color" size="sm" />
      <span class="hidden max-w-[8rem] truncate text-sm text-slate-700 sm:inline dark:text-slate-200">
        {{ user.display_name }}
      </span>
      <span class="sr-only sm:hidden">Account menu for {{ user.display_name }}</span>
    </button>
    <div
      v-if="open"
      :id="menuId"
      class="absolute right-0 z-20 mt-2 w-56 rounded-xl bg-white p-1 shadow-lg ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-700"
      data-testid="account-menu"
    >
      <div class="px-3 py-2">
        <p class="truncate text-sm font-medium text-slate-900 dark:text-slate-50">{{ user.display_name }}</p>
        <p class="truncate text-xs text-slate-500 dark:text-slate-400">@{{ user.handle }}</p>
      </div>
      <ul class="border-t border-slate-100 pt-1 dark:border-slate-800">
        <li>
          <NuxtLink
            :to="`/u/${user.handle}`"
            :class="itemClass"
            data-menu-item
            data-testid="account-menu-profile"
            @click="close()"
          >
            Your profile
          </NuxtLink>
        </li>
        <li>
          <NuxtLink
            to="/my/submissions"
            :class="itemClass"
            data-menu-item
            data-testid="account-menu-submissions"
            @click="close()"
          >
            My submissions
          </NuxtLink>
        </li>
        <li v-if="isAdmin">
          <NuxtLink
            to="/admin/moderation"
            :class="itemClass"
            data-menu-item
            data-testid="account-menu-moderation"
            @click="close()"
          >
            Moderation
          </NuxtLink>
        </li>
        <li>
          <NuxtLink
            to="/settings"
            :class="itemClass"
            data-menu-item
            data-testid="account-menu-settings"
            @click="close()"
          >
            Settings
          </NuxtLink>
        </li>
        <li>
          <button type="button" :class="itemClass" data-menu-item data-testid="logout-button" @click="onLogout">
            Sign out
          </button>
        </li>
      </ul>
    </div>
  </div>
  <div v-else class="flex items-center gap-2">
    <DevSignIn v-if="showDev" variant="compact" />
    <a
      v-if="showGoogle"
      :href="loginURL()"
      class="inline-flex h-9 items-center rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white"
      data-testid="signin-link"
    >
      Sign in with Google
    </a>
  </div>
</template>
