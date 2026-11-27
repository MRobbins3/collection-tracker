import type {
  HandleAvailability,
  Profile,
  PublicCollectionResponse,
  PublicProfileResponse,
} from "~/types/api";

/** Your own profile and privacy settings (GET /me/profile), signed-in only. */
export function useMyProfile() {
  const api = useApi();
  const { isSignedIn, server, watch } = signedInFetchOptions();
  return useAsyncData<Profile | null>(
    "me:profile",
    async () => {
      if (!isSignedIn.value) return null;
      const { profile } = await api.get<{ profile: Profile }>("/me/profile");
      return profile;
    },
    { server, watch, default: () => null },
  );
}

export function useProfileActions() {
  const { publicBaseURL } = useApi();
  const api = useApi();
  const { user } = useAuth();

  /** PATCH /me/profile. Rejects with the API's {error, fields} body on validation errors. */
  async function update(patch: Partial<Omit<Profile, "avatar_color">>): Promise<Profile> {
    const { profile } = await $fetch<{ profile: Profile }>(`${publicBaseURL}/me/profile`, {
      method: "PATCH",
      credentials: "include",
      body: patch,
    });
    // Keep the header (name, avatar, profile link) in step.
    if (user.value) {
      user.value = { ...user.value, handle: profile.handle, display_name: profile.display_name };
    }
    return profile;
  }

  async function checkHandle(handle: string): Promise<HandleAvailability> {
    return await api.get<HandleAvailability>("/me/handle-availability", { handle });
  }

  /** DELETE /me. The API requires the handle again and clears the session. */
  async function deleteAccount(confirmHandle: string): Promise<void> {
    await $fetch(`${publicBaseURL}/me`, {
      method: "DELETE",
      credentials: "include",
      body: { confirm_handle: confirmHandle },
    });
    user.value = null;
  }

  return { update, checkHandle, deleteAccount };
}

/**
 * What the viewer may see of someone's profile. The answer depends on who's
 * asking and the session cookie never reaches SSR, so this loads in the
 * browser once auth has resolved (no anonymous answer flashing before the
 * signed-in one). A hidden or unknown profile resolves to null (the API's 404).
 */
export function usePublicProfile(handle: string) {
  const api = useApi();
  const { ready, isSignedIn } = useAuth();
  return useAsyncData<PublicProfileResponse | null>(
    `profile:${handle}`,
    async () => {
      if (!ready.value) return null;
      try {
        return await api.get<PublicProfileResponse>(`/users/${encodeURIComponent(handle)}`);
      } catch (e) {
        if (isNotFound(e)) return null;
        throw e;
      }
    },
    { server: false, watch: [ready, isSignedIn], default: () => null },
  );
}

/** One collection on someone's profile, read-only. null when hidden or unknown. */
export function usePublicCollection(handle: string, id: string) {
  const api = useApi();
  const { ready, isSignedIn } = useAuth();
  return useAsyncData<PublicCollectionResponse | null>(
    `profile:${handle}:collection:${id}`,
    async () => {
      if (!ready.value) return null;
      try {
        return await api.get<PublicCollectionResponse>(
          `/users/${encodeURIComponent(handle)}/collections/${encodeURIComponent(id)}`,
        );
      } catch (e) {
        if (isNotFound(e)) return null;
        throw e;
      }
    },
    { server: false, watch: [ready, isSignedIn], default: () => null },
  );
}

function isNotFound(e: unknown): boolean {
  return (e as { statusCode?: number; status?: number })?.statusCode === 404 ||
    (e as { status?: number })?.status === 404;
}
