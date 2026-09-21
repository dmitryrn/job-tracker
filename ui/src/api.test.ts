import { afterEach, describe, expect, it, vi } from "vitest";
import { apiURL, fetchJobMatches, fetchJobs, rejectJob } from "./api";

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

describe("fetchJobMatches", () => {
    it("sends the minimum score, sort, viewed and applied filters, and pagination values", async () => {
    vi.stubGlobal("window", { location: { origin: "http://10.8.0.4:4000" } });
    const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ matches: [], total: 0 }) });
    vi.stubGlobal("fetch", fetch);

    await fetchJobMatches(75, "score-desc", 50, 100, new AbortController().signal, "seen", "applied");

    expect(fetch).toHaveBeenCalledWith(
      "http://10.8.0.4:4000/api/matches?sort=score-desc&limit=50&offset=100&minimumScore=75&viewed=seen&applied=applied",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });
});

describe("rejectJob", () => {
	it("sends the reason for not applying", async () => {
		vi.stubGlobal("window", { location: { origin: "http://10.8.0.4:4000" } });
		const fetch = vi.fn().mockResolvedValue({ ok: true });
		vi.stubGlobal("fetch", fetch);

		await rejectJob(42, "Only onsite in Berlin");

		expect(fetch).toHaveBeenCalledWith(
			"http://10.8.0.4:4000/api/jobs/42/rejection",
			{ method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ reason: "Only onsite in Berlin" }) },
		);
	});
});
