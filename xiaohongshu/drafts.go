package xiaohongshu

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

// LocalDraft IndexedDB 中的本地草稿简要信息
type LocalDraft struct {
	DraftID        string `json:"draft_id"`
	UID            string `json:"uid,omitempty"`
	Type           string `json:"type"`
	Title          string `json:"title,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
	ImageCount     int    `json:"image_count,omitempty"`
	UpdatedAt      int64  `json:"updated_at,omitempty"`
}

type DraftAction struct {
	page *rod.Page
}

func NewDraftAction(page *rod.Page) *DraftAction {
	pp := page.Timeout(120 * time.Second)
	return &DraftAction{page: pp}
}

// ListLocalDrafts 从 creator.xiaohongshu.com 的 IndexedDB(draft-database-v1) 读取本地草稿。
// draftType 支持: image|video|article|audio，limit<=0 表示不限。
func (d *DraftAction) ListLocalDrafts(ctx context.Context, draftType string, limit int) ([]LocalDraft, error) {
	page := d.page.Context(ctx)

	// 先导航到创作中心，确保同源上下文可访问 IndexedDB
	page.MustNavigate("https://creator.xiaohongshu.com/").MustWaitLoad()

	result, err := page.Eval(`(draftType, limit) => {
		const storeNameMap = {
			image: "image-draft",
			video: "video-draft",
			article: "article-draft",
			audio: "audio-draft",
		};
		const storeName = storeNameMap[draftType] || "image-draft";
		const maxCount = Number.isFinite(limit) ? limit : 0;

		const pick = (obj, path, defVal = "") => {
			try {
				const parts = path.split(".");
				let cur = obj;
				for (const p of parts) {
					if (cur == null) return defVal;
					cur = cur[p];
				}
				return cur == null ? defVal : cur;
			} catch {
				return defVal;
			}
		};

		return new Promise((resolve, reject) => {
			const req = indexedDB.open("draft-database-v1");
			req.onerror = () => reject(new Error("open draft-database-v1 failed"));
			req.onsuccess = () => {
				try {
					const db = req.result;
					if (!db.objectStoreNames.contains(storeName)) {
						resolve(JSON.stringify([]));
						return;
					}

					const tx = db.transaction(storeName, "readonly");
					const store = tx.objectStore(storeName);
					const items = [];

					const cursorReq = store.openCursor();
					cursorReq.onerror = () => reject(new Error("openCursor failed"));
					cursorReq.onsuccess = (e) => {
						const cursor = e.target.result;
						if (!cursor) {
							items.sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0));
							const sliced = maxCount > 0 ? items.slice(0, maxCount) : items;
							resolve(JSON.stringify(sliced));
							return;
						}

						const key = cursor.key;
						const v = cursor.value || {};
						const content = v.content || {};
						const articleStore = content.articleStore || {};
						const draftStore = content.draftStore || {};
						const imgList = Array.isArray(draftStore.imgList) ? draftStore.imgList : [];

						const title = String(
							articleStore.articleTitle ||
							draftStore.title ||
							v.title ||
							"",
						);
						const body = String(
							articleStore.articleContent ||
							draftStore.desc ||
							v.content ||
							"",
						);

						items.push({
							draft_id: String(v.draftId || key || ""),
							uid: String(v.uid || ""),
							type: String(draftType),
							title,
							content_preview: body ? body.slice(0, 120) : "",
							image_count: imgList.length,
							updated_at: Number(v.timeStamp || v.updateTime || v.updatedAt || 0),
						});
						cursor.continue();
					};
				} catch (e) {
					reject(e);
				}
			};
		});
	}`, draftType, limit)
	if err != nil {
		return nil, errors.Wrap(err, "读取草稿 IndexedDB 失败")
	}

	raw := result.Value.String()
	var drafts []LocalDraft
	if err := json.Unmarshal([]byte(raw), &drafts); err != nil {
		return nil, errors.Wrap(err, "解析草稿数据失败")
	}
	return drafts, nil
}

// GetLocalDraftDetail 按 draft_id 从 draft-database-v1 读取单条草稿完整内容（value 已做 JSON 友好化，File/Blob 转为摘要）。
// draftType 为空则在 image-draft、video-draft、article-draft、audio-draft 中依次查找；否则只查对应 store。
func (d *DraftAction) GetLocalDraftDetail(ctx context.Context, draftID, draftType string) (json.RawMessage, error) {
	if draftID == "" {
		return nil, errors.New("draft_id 不能为空")
	}
	page := d.page.Context(ctx)
	page.MustNavigate("https://creator.xiaohongshu.com/").MustWaitLoad()

	result, err := page.Eval(`(draftId, draftType) => {
		const storeNameMap = {
			image: "image-draft",
			video: "video-draft",
			article: "article-draft",
			audio: "audio-draft",
		};
		const order = (draftType && storeNameMap[draftType])
			? [storeNameMap[draftType]]
			: ["image-draft", "video-draft", "article-draft", "audio-draft"];

		function logicalType(storeName) {
			for (const k of Object.keys(storeNameMap)) {
				if (storeNameMap[k] === storeName) return k;
			}
			return "";
		}

		function sanitize(v) {
			if (v == null) return v;
			const t = typeof v;
			if (t === "function" || t === "symbol") return undefined;
			if (v instanceof File) return { __kind: "File", name: v.name, size: v.size, type: v.type };
			if (v instanceof Blob) return { __kind: "Blob", size: v.size, type: v.type };
			if (Array.isArray(v)) return v.map(sanitize).filter(x => x !== undefined);
			if (t === "object") {
				const o = {};
				for (const k of Object.keys(v)) {
					try {
						const s = sanitize(v[k]);
						if (s !== undefined) o[k] = s;
					} catch (e) {}
				}
				return o;
			}
			return v;
		}

		return new Promise((resolve, reject) => {
			const openReq = indexedDB.open("draft-database-v1");
			openReq.onerror = () => reject(new Error("open draft-database-v1 failed"));
			openReq.onsuccess = () => {
				const db = openReq.result;
				function tryNext(i) {
					if (i >= order.length) {
						db.close();
						resolve(JSON.stringify({ found: false, draft_id: String(draftId) }));
						return;
					}
					const storeName = order[i];
					if (!db.objectStoreNames.contains(storeName)) {
						tryNext(i + 1);
						return;
					}
					const tx = db.transaction(storeName, "readonly");
					const store = tx.objectStore(storeName);
					const g = store.get(draftId);
					g.onerror = () => { try { db.close(); } catch (e) {} reject(g.error); };
					g.onsuccess = () => {
						let val = g.result;
						if (val != null) {
							db.close();
							resolve(JSON.stringify({
								found: true,
								store: storeName,
								draft_id: String(draftId),
								type: logicalType(storeName),
								value: sanitize(val),
							}));
							return;
						}
						const cur = store.openCursor();
						cur.onerror = () => { try { db.close(); } catch (e) {} reject(cur.error); };
						cur.onsuccess = (e) => {
							const c = e.target.result;
							if (!c) {
								tryNext(i + 1);
								return;
							}
							const v = c.value || {};
							if (String(v.draftId || "") === String(draftId) || String(c.key || "") === String(draftId)) {
								db.close();
								resolve(JSON.stringify({
									found: true,
									store: storeName,
									draft_id: String(draftId),
									type: logicalType(storeName),
									value: sanitize(v),
								}));
								return;
							}
							c.continue();
						};
					};
				}
				tryNext(0);
			};
		});
	}`, draftID, draftType)
	if err != nil {
		return nil, errors.Wrap(err, "读取草稿详情失败")
	}
	raw := result.Value.String()
	if raw == "" {
		return nil, errors.New("未拿到草稿详情")
	}
	return json.RawMessage(raw), nil
}

