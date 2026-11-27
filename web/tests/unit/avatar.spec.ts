import { describe, it, expect } from "vitest";
import { AVATAR_CLASSES, avatarClass, initials } from "~/utils/avatar";

describe("initials", () => {
  it("uses the first letters of the first and last words", () => {
    expect(initials("Liam Johnson")).toBe("LJ");
    expect(initials("  olivia   de la cruz ")).toBe("OC");
    expect(initials("Emma")).toBe("E");
    expect(initials("josé garcía")).toBe("JG");
  });

  it("falls back to a placeholder for blank names", () => {
    expect(initials("")).toBe("?");
    expect(initials("   ")).toBe("?");
  });

  it("doesn't split emoji or non-Latin characters in half", () => {
    expect(initials("渡辺 健")).toBe("渡健");
    expect(initials("🦖 Fan")).toBe("🦖F");
  });
});

describe("avatarClass", () => {
  it("maps every API palette color to classes", () => {
    for (const c of ["rose", "orange", "amber", "emerald", "teal", "sky", "indigo", "violet", "fuchsia", "slate"]) {
      expect(AVATAR_CLASSES[c]).toBeTruthy();
      expect(avatarClass(c)).toBe(AVATAR_CLASSES[c]);
    }
  });

  it("falls back to slate for an unknown color", () => {
    expect(avatarClass("chartreuse")).toBe(AVATAR_CLASSES.slate);
    expect(avatarClass(undefined)).toBe(AVATAR_CLASSES.slate);
  });
});
