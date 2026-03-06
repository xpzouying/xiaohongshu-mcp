import { describe, expect, it } from "vitest";
import { parseKeywords, parseManualCandidates } from "./utils";

describe("parseKeywords", () => {
  it("splits comma and line break separated input", () => {
    expect(parseKeywords("穿搭,美妆\n效率")).toEqual(["穿搭", "美妆", "效率"]);
  });
});

describe("parseManualCandidates", () => {
  it("parses feed_id:xsec_token rows", () => {
    expect(parseManualCandidates("feed1:token1\nfeed2:token2")).toEqual([
      { feed_id: "feed1", xsec_token: "token1" },
      { feed_id: "feed2", xsec_token: "token2" }
    ]);
  });

  it("throws on malformed row", () => {
    expect(() => parseManualCandidates("feed-only")).toThrowError();
  });
});

