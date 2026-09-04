import { afterEach, describe, expect, it, vi } from "vitest";
import { apiURL } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("apiURL", () => {
  it("uses the current origin for database downloads", () => {
    vi.stubGlobal("window", { location: { origin: "http://10.8.0.4:4000" } });

    expect(apiURL("database")).toBe("http://10.8.0.4:4000/api/database");
  });
});
