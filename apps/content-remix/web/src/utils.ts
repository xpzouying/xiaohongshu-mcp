export type ManualCandidate = {
  feed_id: string;
  xsec_token: string;
};

export function parseKeywords(value: string): string[] {
  return value
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function parseManualCandidates(value: string): ManualCandidate[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [feedId, xsecToken] = line.split(":").map((item) => item.trim());
      if (!feedId || !xsecToken) {
        throw new Error(`invalid manual candidate line: ${line}`);
      }
      return { feed_id: feedId, xsec_token: xsecToken };
    });
}

