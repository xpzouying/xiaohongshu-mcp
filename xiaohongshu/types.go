package xiaohongshu

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
	Video        *FeedDetailVideo  `json:"video,omitempty"`
}

// FeedDetailVideo 表示详情页的视频信息
type FeedDetailVideo struct {
	// URL 是可直接下载的视频地址（从 media.stream 中提取的最高质量 URL）
	URL      string                `json:"url,omitempty"`
	Duration int                   `json:"duration,omitempty"`
	Media    *FeedDetailVideoMedia `json:"media,omitempty"`
}

// FeedDetailVideoMedia 表示视频媒体信息
type FeedDetailVideoMedia struct {
	Stream map[string][]VideoStream `json:"stream,omitempty"`
}

// VideoStream 表示单个视频流
type VideoStream struct {
	MasterURL  string   `json:"masterUrl,omitempty"`
	BackupURLs []string `json:"backupUrls,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	AvgBitrate int      `json:"avgBitrate,omitempty"`
}

// 视频编码优先级（h264 兼容性最好）
var videoCodecPriority = []string{"h264", "h265", "av1", "h266"}

// 小红书视频 CDN 基础地址
const xhsVideoCDNBase = "https://sns-video-bd.xhscdn.com/"

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
