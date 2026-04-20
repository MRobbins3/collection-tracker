export type ThemePreference = "system" | "light" | "dark";
export type EffectiveTheme = "light" | "dark";

const STORAGE_KEY = "ct-theme";
const DARK_QUERY = "(prefers-color-scheme: dark)";

/**
 * useTheme owns the three-state theme preference ('system' | 'light' | 'dark')
 * and the resolved effective theme that actually drives CSS classes. The
 * pre-hydration inline script in nuxt.config.ts already set the correct `dark`
 * class on <html> before first paint; this composable keeps it in sync with
 * user actions and system-preference changes thereafter.
 */
export function useTheme() {
  const preference = useState<ThemePreference>("theme:preference", () => "system");
  const systemDark = useState<boolean>("theme:systemDark", () => false);

  const effective = computed<EffectiveTheme>(() =>
    preference.value === "system" ? (systemDark.value ? "dark" : "light") : preference.value,
  );

  function applyClass(eff: EffectiveTheme) {
    if (import.meta.server) return;
    document.documentElement.classList.toggle("dark", eff === "dark");
  }

  function hydrateFromBrowser() {
    if (import.meta.server) return;
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      preference.value = saved === "light" || saved === "dark" ? saved : "system";
    } catch {
      preference.value = "system";
    }
    systemDark.value = window.matchMedia(DARK_QUERY).matches;
    applyClass(effective.value);
    const mq = window.matchMedia(DARK_QUERY);
    mq.addEventListener?.("change", (e) => {
      systemDark.value = e.matches;
      applyClass(effective.value);
    });
  }

  function setPreference(p: ThemePreference) {
    preference.value = p;
    if (import.meta.client) {
      try {
        localStorage.setItem(STORAGE_KEY, p);
      } catch {
        /* storage disabled — in-memory only */
      }
      applyClass(effective.value);
    }
  }

  // Cycle order: system → light → dark → system. Easy to memorize.
  function cycle() {
    const next: ThemePreference =
      preference.value === "system" ? "light" : preference.value === "light" ? "dark" : "system";
    setPreference(next);
  }

  return { preference, effective, systemDark, hydrateFromBrowser, setPreference, cycle };
}
