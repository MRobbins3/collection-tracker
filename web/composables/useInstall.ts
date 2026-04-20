/**
 * PWA install plumbing.
 *
 * - Android/Chromium fires `beforeinstallprompt`; we stash it so the UI can
 *   surface a one-tap install button when appropriate.
 * - iOS Safari never fires that event — the only path is the Share sheet's
 *   "Add to Home Screen." For iOS users we show an instructional card instead.
 *
 * The composable is SSR-safe: all UA sniffing + event listening is guarded
 * by `import.meta.client` so first render matches server output.
 */
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

const DISMISS_KEY = "ct-install-dismissed";

export function useInstall() {
  const deferredPrompt = useState<BeforeInstallPromptEvent | null>(
    "install:prompt",
    () => null,
  );
  const dismissed = useState<boolean>("install:dismissed", () => false);
  const isStandalone = useState<boolean>("install:standalone", () => false);
  const isIOS = useState<boolean>("install:ios", () => false);

  function hydrate() {
    if (import.meta.server) return;

    try {
      dismissed.value = localStorage.getItem(DISMISS_KEY) === "1";
    } catch {
      /* storage disabled — treat as not dismissed */
    }

    isStandalone.value =
      window.matchMedia("(display-mode: standalone)").matches ||
      // Safari-specific property (legacy but still present on iOS).
      (window.navigator as { standalone?: boolean }).standalone === true;

    const ua = window.navigator.userAgent;
    isIOS.value = /iPad|iPhone|iPod/.test(ua) && !("MSStream" in window);

    window.addEventListener("beforeinstallprompt", (e) => {
      e.preventDefault();
      deferredPrompt.value = e as BeforeInstallPromptEvent;
    });
    window.addEventListener("appinstalled", () => {
      deferredPrompt.value = null;
      isStandalone.value = true;
    });
  }

  async function promptInstall(): Promise<"accepted" | "dismissed" | "unsupported"> {
    const evt = deferredPrompt.value;
    if (!evt) return "unsupported";
    await evt.prompt();
    const { outcome } = await evt.userChoice;
    deferredPrompt.value = null;
    return outcome;
  }

  function dismiss(): void {
    dismissed.value = true;
    if (import.meta.client) {
      try {
        localStorage.setItem(DISMISS_KEY, "1");
      } catch {
        /* ignore */
      }
    }
  }

  // Three shapes of the install surface:
  // - 'android' → browser-provided prompt available, show a single install button
  // - 'ios'     → explain the Share-sheet flow (no programmatic prompt)
  // - 'hidden'  → already installed, dismissed, or on an unsupported browser
  const surface = computed<"android" | "ios" | "hidden">(() => {
    if (isStandalone.value || dismissed.value) return "hidden";
    if (deferredPrompt.value) return "android";
    if (isIOS.value) return "ios";
    return "hidden";
  });

  return { surface, promptInstall, dismiss, hydrate };
}
