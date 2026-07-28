package xiaohongshu

import "encoding/json"

// 小红书 Feed 相关的数据结构定义

// FeedResponse 表示从 __INITIAL_STATE__ 中获取的完整 Feed 响应
type FeedResponse struct {
	Feed FeedData `json:"feed"`
}

// FeedData 表示 feed 数据结构
type FeedData struct {
	Feeds FeedsValue `json:"feeds"`
}

// FeedsValue 表示 feeds 的值结构
type FeedsValue struct {
	Value []Feed `json:"_value"`
}

// Feed 表示单个 Feed 项目
type Feed struct {
	XsecToken string   `json:"xsecToken"`
	ID        string   `json:"id"`
	ModelType string   `json:"modelType"`
	NoteCard  NoteCard `json:"noteCard"`
	Index     int      `json:"index"`
}

// modelTypeNote 笔记条目的 modelType 取值。
const modelTypeNote = "note"

// onlyNotes 滤掉非笔记条目。
//
// 站点返回的列表里混着直播卡片（live_v2）和搜索热词（hot_query），它们没有
// noteCard，取出来标题为空、也没有可用的 id，对调用方是纯噪音。
//
// 判据用 modelType 而非 noteCard.type：图文与视频笔记的 modelType 同为 note，
// 差异体现在 noteCard.type（normal / video），按 modelType 过滤不会误伤视频。
func onlyNotes(feeds []Feed) []Feed {
	notes := make([]Feed, 0, len(feeds))
	for _, f := range feeds {
		if f.ModelType == modelTypeNote {
			notes = append(notes, f)
		}
	}
	return notes
}

// NoteCard 表示笔记卡片信息
type NoteCard struct {
	Type         string       `json:"type"`
	DisplayTitle string       `json:"displayTitle"`
	User         User         `json:"user"`
	InteractInfo InteractInfo `json:"interactInfo"`
	Cover        Cover        `json:"cover"`
	Video        *Video       `json:"video,omitempty"` // 视频内容，可能为空
}

// User 表示用户信息
type User struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	NickName string `json:"nickName"`
	Avatar   string `json:"avatar"`
}

// InteractInfo 表示互动信息
type InteractInfo struct {
	Liked      bool   `json:"liked"`
	LikedCount string `json:"likedCount"`

	SharedCount  string `json:"sharedCount"`
	CommentCount string `json:"commentCount"`

	CollectedCount string `json:"collectedCount"`
	Collected      bool   `json:"collected"`
}

// Cover 表示封面信息
type Cover struct {
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	URL        string      `json:"url"`
	FileID     string      `json:"fileId"`
	URLPre     string      `json:"urlPre"`
	URLDefault string      `json:"urlDefault"`
	InfoList   []ImageInfo `json:"infoList"`
}

// ImageInfo 表示图片信息
type ImageInfo struct {
	ImageScene string `json:"imageScene"`
	URL        string `json:"url"`
}

// Video 表示视频信息
type Video struct {
	Capa VideoCapability `json:"capa"`
}

// VideoCapability 表示视频能力信息
type VideoCapability struct {
	Duration int `json:"duration"` // 视频时长，单位秒
}

// ================ Feed 详情页相关结构体 ================

// FeedDetailResponse 表示 Feed 详情页完整响应
type FeedDetailResponse struct {
	Note     FeedDetail  `json:"note"`
	Comments CommentList `json:"comments"`
}

// FeedDetail 表示详情页的笔记内容
type FeedDetail struct {
	NoteID       string            `json:"noteId"`
	XsecToken    string            `json:"xsecToken"`
	Title        string            `json:"title"`
	Desc         string            `json:"desc"`
	Type         string            `json:"type"`
	Time         int64             `json:"time"`
	IPLocation   string            `json:"ipLocation"`
	User         User              `json:"user"`
	InteractInfo InteractInfo      `json:"interactInfo"`
	ImageList    []DetailImageInfo `json:"imageList"`
	Video        *VideoDetail      `json:"video,omitempty"` // 视频笔记才有，图文笔记为 nil
}

// VideoDetail 详情页的视频信息，按页面 note.video 原样映射，不替调用方挑档位。
type VideoDetail struct {
	Image VideoImage      `json:"image"`
	Capa  VideoCapability `json:"capa"`
	Media VideoMedia      `json:"media"`
	// Subtitles 字幕，从 mediaV2 里解出来（见 UnmarshalJSON）。key 为语言，另有 source 表示原始语种。
	Subtitles map[string][]VideoSubtitle `json:"subtitles,omitempty"`
}

// UnmarshalJSON 额外解开 mediaV2。
// mediaV2 是 media 的字符串化副本，字段基本重复，唯独字幕只在它里面有，
// 所以只取字幕，不把整个副本塞进返回体。
func (v *VideoDetail) UnmarshalJSON(data []byte) error {
	type alias VideoDetail // 借别名避免递归调用本方法
	aux := struct {
		MediaV2 string `json:"mediaV2"`
		*alias
	}{alias: (*alias)(v)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.MediaV2 == "" {
		return nil
	}

	var v2 struct {
		Video struct {
			Subtitles map[string][]VideoSubtitle `json:"subtitles"`
		} `json:"video"`
	}
	// 字幕属于附加信息，解不出不影响视频地址，静默跳过
	if err := json.Unmarshal([]byte(aux.MediaV2), &v2); err == nil {
		v.Subtitles = v2.Video.Subtitles
	}
	return nil
}

// VideoImage 视频的首帧与缩略图，只有 fileid，需自行拼 CDN 地址。
type VideoImage struct {
	FirstFrameFileID string `json:"firstFrameFileid"`
	ThumbnailFileID  string `json:"thumbnailFileid"`
}

// VideoMedia 视频媒体信息。
type VideoMedia struct {
	VideoID int64     `json:"videoId"`
	Video   VideoMeta `json:"video"`
	// Stream 按编码名分桶：h264/h265/av1/h266，同一编码下可有多档分辨率，也可能是空数组。
	// 用 map 而不是写死字段，是为了小红书新增编码时不会被静默丢掉。
	Stream map[string][]VideoStream `json:"stream"`
}

// VideoMeta 视频的整体信息。注意 Duration 是秒（四舍五入），精确时长看 VideoStream.Duration。
type VideoMeta struct {
	Duration    int    `json:"duration"`
	MD5         string `json:"md5"`
	HDRType     int    `json:"hdrType"`
	DRMType     int    `json:"drmType"`
	StreamTypes []int  `json:"streamTypes"`
	BizName     int    `json:"bizName"`
	BizID       string `json:"bizId"`
}

// VideoStream 单档视频流。
// MasterURL 带 sign 与 t（过期时间戳）签名参数，有时效；BackupURLs 不带签名。
type VideoStream struct {
	MasterURL  string   `json:"masterUrl"`
	BackupURLs []string `json:"backupUrls"`

	Format      string `json:"format"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Duration    int    `json:"duration"` // 毫秒
	Size        int64  `json:"size"`     // 字节
	FPS         int    `json:"fps"`
	Rotate      int    `json:"rotate"`
	QualityType string `json:"qualityType"`
	StreamType  int    `json:"streamType"`
	StreamDesc  string `json:"streamDesc"`
	HDRType     int    `json:"hdrType"`

	VideoCodec    string `json:"videoCodec"`
	VideoBitrate  int    `json:"videoBitrate"`
	VideoDuration int    `json:"videoDuration"`
	AvgBitrate    int    `json:"avgBitrate"`

	AudioCodec    string  `json:"audioCodec"`
	AudioBitrate  int     `json:"audioBitrate"`
	AudioDuration int     `json:"audioDuration"`
	AudioChannels int     `json:"audioChannels"`
	Volume        float64 `json:"volume"`

	// 转码质量指标，小红书内部用
	VMAF          float64 `json:"vmaf"`
	PSNR          float64 `json:"psnr"`
	SSIM          float64 `json:"ssim"`
	Weight        int     `json:"weight"`
	DefaultStream int     `json:"defaultStream"`
}

// VideoSubtitle 字幕文件，URL 为 .srt 直链，同样带签名有时效。
type VideoSubtitle struct {
	URL      string `json:"url"`
	Language string `json:"language"`
	Format   int    `json:"format"`
	Type     int    `json:"type"`
}

// DetailImageInfo 表示详情页的图片信息
type DetailImageInfo struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	URLDefault string `json:"urlDefault"`
	URLPre     string `json:"urlPre"`
	LivePhoto  bool   `json:"livePhoto,omitempty"`
}

// CommentList 表示评论列表
type CommentList struct {
	List    []Comment `json:"list"`
	Cursor  string    `json:"cursor"`
	HasMore bool      `json:"hasMore"`
}

// Comment 表示单条评论
type Comment struct {
	ID              string    `json:"id"`
	NoteID          string    `json:"noteId"`
	Content         string    `json:"content"`
	LikeCount       string    `json:"likeCount"`
	CreateTime      int64     `json:"createTime"`
	IPLocation      string    `json:"ipLocation"`
	Liked           bool      `json:"liked"`
	UserInfo        User      `json:"userInfo"`
	SubCommentCount string    `json:"subCommentCount"`
	SubComments     []Comment `json:"subComments"`
	ShowTags        []string  `json:"showTags"`
}

// UserProfileResponse 用户详情页完整响应
type UserProfileResponse struct {
	UserBasicInfo UserBasicInfo      `json:"userBasicInfo"`
	Interactions  []UserInteractions `json:"interactions"`
	Feeds         []Feed             `json:"feeds"`
}

// UserPageData 用户的详细信息
type UserPageData struct {
	RawValue struct {
		Interactions []UserInteractions `json:"interactions"`
		BasicInfo    UserBasicInfo      `json:"basicInfo"`
	} `json:"_rawValue"`
}

// UserBasicInfo 用户的基本信息
type UserBasicInfo struct {
	Gender     int    `json:"gender"`
	IpLocation string `json:"ipLocation"`
	Desc       string `json:"desc"`
	Imageb     string `json:"imageb"`
	Nickname   string `json:"nickname"`
	Images     string `json:"images"`
	RedId      string `json:"redId"`
}

// UserInteractions 用户的 关注 粉丝 收藏量
type UserInteractions struct {
	Type  string `json:"type"`  // follows fans interaction
	Name  string `json:"name"`  // 关注 粉丝 获赞与收藏
	Count string `json:"count"` // 数量
}
