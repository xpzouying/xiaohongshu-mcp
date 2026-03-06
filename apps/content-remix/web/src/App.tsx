import { useEffect, useMemo, useState } from "react";
import { parseKeywords, parseManualCandidates } from "./utils";

type JobStatus = "idle" | "queued" | "running" | "succeeded" | "failed";

type RemixResult = {
  job_id: string;
  candidate_count: number;
  errors: string[];
  viral_breakdown: Array<Record<string, unknown>>;
  remix_ideas: Array<Record<string, unknown>>;
  candidates: Array<Record<string, unknown>>;
};

const API_BASE = import.meta.env.VITE_REMIX_API_BASE || "http://localhost:18061";

export default function App() {
  const [keywordsInput, setKeywordsInput] = useState("爆款文案,职场效率");
  const [manualInput, setManualInput] = useState("");
  const [candidateLimit, setCandidateLimit] = useState(10);

  const [jobId, setJobId] = useState("");
  const [status, setStatus] = useState<JobStatus>("idle");
  const [result, setResult] = useState<RemixResult | null>(null);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const canSubmit = useMemo(() => !submitting, [submitting]);

  async function submitJob() {
    setError("");
    setResult(null);
    setSubmitting(true);
    try {
      const payload = {
        keywords: parseKeywords(keywordsInput),
        manual_candidates: manualInput.trim() ? parseManualCandidates(manualInput) : [],
        candidate_limit: candidateLimit
      };
      const response = await fetch(`${API_BASE}/api/remix/jobs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      if (!response.ok) {
        throw new Error(`submit failed: ${response.status}`);
      }
      const body = await response.json();
      setJobId(body.job_id);
      setStatus(body.status as JobStatus);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  }

  useEffect(() => {
    if (!jobId) return;
    if (status === "failed" || status === "succeeded") return;

    const timer = setInterval(async () => {
      try {
        const response = await fetch(`${API_BASE}/api/remix/jobs/${jobId}`);
        if (!response.ok) {
          throw new Error(`status failed: ${response.status}`);
        }
        const body = await response.json();
        const nextStatus = body.status as JobStatus;
        setStatus(nextStatus);
        if (nextStatus === "succeeded") {
          const resultResp = await fetch(`${API_BASE}/api/remix/jobs/${jobId}/result`);
          if (!resultResp.ok) {
            throw new Error(`result failed: ${resultResp.status}`);
          }
          const resultBody = await resultResp.json();
          setResult(resultBody);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
        setStatus("failed");
      }
    }, 3000);

    return () => clearInterval(timer);
  }, [jobId, status]);

  return (
    <div className="min-h-screen p-4 md:p-8">
      <div className="mx-auto max-w-6xl space-y-6">
        <header className="rounded-2xl border border-slate-200 bg-white/80 p-6 shadow-sm backdrop-blur-sm">
          <p className="text-sm font-medium uppercase tracking-widest text-brand-primary">ContentRemixAgent</p>
          <h1 className="mt-2 text-3xl font-bold text-brand-ink">小红书二创控制台</h1>
          <p className="mt-2 text-sm text-slate-600">
            输入关键词或手动候选，系统会复用 xiaohongshu-mcp 的搜索、详情和视频转写能力，输出爆款拆解与二创灵感。
          </p>
        </header>

        <section className="grid gap-4 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm md:grid-cols-2">
          <div className="space-y-2">
            <label className="block text-sm font-semibold text-brand-ink">关键词（逗号或换行分隔）</label>
            <textarea
              value={keywordsInput}
              onChange={(event) => setKeywordsInput(event.target.value)}
              rows={5}
              className="w-full rounded-lg border border-slate-300 p-3 text-sm focus:border-brand-primary focus:outline-none"
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-semibold text-brand-ink">手动候选（feed_id:xsec_token）</label>
            <textarea
              value={manualInput}
              onChange={(event) => setManualInput(event.target.value)}
              rows={5}
              placeholder="6834abcd000000000f12a111:xsec-token-here"
              className="w-full rounded-lg border border-slate-300 p-3 text-sm focus:border-brand-primary focus:outline-none"
            />
          </div>

          <div className="flex items-center gap-3">
            <span className="text-sm font-semibold text-brand-ink">候选上限</span>
            <input
              type="number"
              min={1}
              max={20}
              value={candidateLimit}
              onChange={(event) => setCandidateLimit(Number(event.target.value))}
              className="w-24 rounded-lg border border-slate-300 p-2 text-sm"
            />
            <button
              type="button"
              onClick={submitJob}
              disabled={!canSubmit}
              className="rounded-lg bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {submitting ? "提交中..." : "启动任务"}
            </button>
          </div>

          <div className="rounded-lg bg-brand-soft p-3 text-sm">
            <p>
              <strong>任务ID：</strong>
              {jobId || "-"}
            </p>
            <p>
              <strong>状态：</strong>
              {status}
            </p>
          </div>
        </section>

        {error ? (
          <section className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error}</section>
        ) : null}

        {result ? (
          <section className="space-y-4 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <h2 className="text-xl font-bold text-brand-ink">任务结果</h2>
            <p className="text-sm text-slate-600">候选数量：{result.candidate_count}</p>

            <div className="grid gap-4 md:grid-cols-2">
              <article className="rounded-lg border border-slate-200 p-4">
                <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand-primary">爆款拆解</h3>
                <pre className="max-h-96 overflow-auto whitespace-pre-wrap text-xs text-slate-700">
                  {JSON.stringify(result.viral_breakdown, null, 2)}
                </pre>
              </article>
              <article className="rounded-lg border border-slate-200 p-4">
                <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-brand-accent">二创灵感</h3>
                <pre className="max-h-96 overflow-auto whitespace-pre-wrap text-xs text-slate-700">
                  {JSON.stringify(result.remix_ideas, null, 2)}
                </pre>
              </article>
            </div>
          </section>
        ) : null}
      </div>
    </div>
  );
}

