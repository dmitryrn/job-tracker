import { afterEach, describe, expect, it, vi } from "vitest";
import { apiURL, fetchJobs } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("apiURL", () => {
  it("uses the current origin for database downloads", () => {
    vi.stubGlobal("window", { location: { origin: "http://10.8.0.4:4000" } });

    expect(apiURL("database")).toBe("http://10.8.0.4:4000/api/database");
  });
});

describe("fetchJobs", () => {
  it("sends pagination values with the active filters", async () => {
    vi.stubGlobal("window", { location: { origin: "http://10.8.0.4:4000" } });
    const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ jobs: [], total: 0 }) });
    vi.stubGlobal("fetch", fetch);

    await fetchJobs("engineer", "adzuna", "has", ["title", "company"], 50, 100, new AbortController().signal);

    expect(fetch).toHaveBeenCalledWith(
      "http://10.8.0.4:4000/api/jobs?search=engineer&provider=adzuna&match=has&fields=title%2Ccompany&limit=50&offset=100",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });
});
