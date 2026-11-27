import { describe, it, expect } from "vitest";
import { collapseWhitespace, handleProblem, normalizeHandle, suggestTitleCase } from "~/utils/text";

describe("collapseWhitespace", () => {
  it("trims and collapses every run of whitespace", () => {
    expect(collapseWhitespace("  star \n\t wars ")).toBe("star wars");
    expect(collapseWhitespace("   ")).toBe("");
    expect(collapseWhitespace(undefined)).toBe("");
  });
});

describe("suggestTitleCase", () => {
  it("suggests Title Case for an all-lowercase value", () => {
    expect(suggestTitleCase("liam johnson")).toBe("Liam Johnson");
    expect(suggestTitleCase("  star   wars ")).toBe("Star Wars");
  });

  it("stays quiet when the user already chose their casing", () => {
    expect(suggestTitleCase("Liam Johnson")).toBeNull();
    expect(suggestTitleCase("LEGO UCS fan")).toBeNull();
    expect(suggestTitleCase("mcDonald")).toBeNull();
    expect(suggestTitleCase("Liam johnson")).toBeNull();
  });

  it("has nothing to suggest for blank or caseless input", () => {
    expect(suggestTitleCase("")).toBeNull();
    expect(suggestTitleCase("42")).toBeNull();
    expect(suggestTitleCase("渡辺")).toBeNull();
  });

  it("handles accented letters", () => {
    expect(suggestTitleCase("josé garcía")).toBe("José García");
  });
});

describe("normalizeHandle", () => {
  it("trims, drops one leading @, and lowercases (same as the API)", () => {
    expect(normalizeHandle("  @Liam_J ")).toBe("liam_j");
    expect(normalizeHandle("@@x")).toBe("@x");
  });
});

describe("handleProblem", () => {
  it("accepts valid handles", () => {
    for (const h of ["liam", "liam_johnson", "a12", "x".repeat(30)]) {
      expect(handleProblem(h)).toBeNull();
    }
  });

  it("explains what's wrong, in the API's words", () => {
    expect(handleProblem("ab")).toBe("Handles are 3–30 characters.");
    expect(handleProblem("x".repeat(31))).toBe("Handles are 3–30 characters.");
    expect(handleProblem("liam johnson")).toBe("Use only lowercase letters, numbers, and underscores.");
    expect(handleProblem("liam-j")).toBe("Use only lowercase letters, numbers, and underscores.");
    expect(handleProblem("1liam")).toBe("Start your handle with a letter.");
  });
});
