package wxweb

// const about wx
const (
	MSG_TYPE_TEXT        = 1
	MSG_TYPE_IMG         = 3
	MSG_TYPE_INIT        = 51
	MSG_TYPE_SYSTEM      = 10000
	MSG_TYPE_VERIFY_USER = 37
	MSG_TYPE_VIDEO       = 43
	MSG_TYPE_VOICE       = 34
	MSG_TYPE_CARD        = 42
	MSG_TYPE_SHARE_URL   = 49
	
	MSG_TYPE_TRANSFER    = 2000
)

const (
	WX_RET_SUCCESS = iota
)

const (
	WEBWX_SYNC_INTERVAL            = 2
	WEBWX_HANDLE_MSG_SYNC_INTERVAL = 1
)

const (
	GROUP_PREFIX = "@@"
)

const (
	WX_BOY   = 1
	WX_GIRL  = 2
	WX_OTHER = 0
)

const (
	FROM_TYPE_PEOPLE                   = "people"
	FROM_TYPE_GROUP                    = "group"
	RECEIVE_EVENT_MSG                  = "receivemsg"
	RECEIVE_EVENT_MOD_GROUP_ADD        = "modgroupadd"
	RECEIVE_EVENT_MOD_GROUP_ADD_DETAIL = "modgroupadddetail"
	RECEIVE_EVENT_ADD_FRIEND           = "addfriend"
	RECEIVE_EVENT_ADD                  = "receiveadd"
	RECEIVE_EVENT_ADD_GROUP            = "addgroup"
)

const (
	RECEIVE_MSG_TYPE_TEXT       = "text"
	RECEIVE_MSG_TYPE_IMG        = "img"
	RECEIVE_MSG_TYPE_VOICE      = "voice"
	RECEIVE_MSG_TYPE_VIDEO      = "video"
	RECEIVE_MSG_TYPE_CARD       = "card"
	RECEIVE_MSG_TYPE_SHARE      = "shareurl"
	RECEIVE_MSG_TYPE_TRANSFER   = "transfer"   // 转账
	RECEIVE_MSG_TYPE_RED_PACKET = "red-packet" // 红包
)

type msgUrlHandle func(string) string

var (
	RECEIVE_MSG_MAP = map[int]string{
		MSG_TYPE_TEXT:      RECEIVE_MSG_TYPE_TEXT,
		MSG_TYPE_IMG:       RECEIVE_MSG_TYPE_IMG,
		MSG_TYPE_VOICE:     RECEIVE_MSG_TYPE_VOICE,
		MSG_TYPE_VIDEO:     RECEIVE_MSG_TYPE_VIDEO,
		MSG_TYPE_CARD:      RECEIVE_MSG_TYPE_CARD,
		MSG_TYPE_SHARE_URL: RECEIVE_MSG_TYPE_SHARE,
		MSG_TYPE_TRANSFER:  RECEIVE_MSG_TYPE_TRANSFER,
	}
	RECEIVE_MSG_CONTENT_MAP = map[int]string{
		MSG_TYPE_IMG:       "收到一张图片,URL为临时地址,当前登录状态下有效(访问需带上cookie)",
		MSG_TYPE_VOICE:     "收到一段语音,URL为临时地址,当前登录状态下有效(访问需带上cookie)",
		MSG_TYPE_VIDEO:     "收到一段视频,URL为临时地址,当前登录状态下有效(访问需带上cookie)",
		MSG_TYPE_CARD:      "收到分享名片",
		MSG_TYPE_SHARE_URL: "收到分享链接",
		MSG_TYPE_TRANSFER:  "收到转账消息，请在手机上查看",
	}
)

const (
	WX_VERIFY_USER_OP_ADD     = 2
	WX_VERIFY_USER_OP_CONFIRM = 3
)

const (
	MSG_MEDIA_KEYWORD        = "CDATA"
	CLEAR_WX_PREFIX_DEFAULT  = "A已被删除"
	WX_SYSTEM_NOT_FRIEND     = "开启了朋友验证"
	WX_SYSTEM_MSG_INVITE     = "邀请"
	WX_SYSTEM_MSG_SCAN       = "扫描"
	WX_SYSTEM_MSG_RED_PACKET = "红包"
)

const (
	WX_FRIEND_VERIFY_FLAG_USER       = 0
	WX_FRIEND_VERIFY_FLAG_DINGYUEHAO = 8
	WX_FRIEND_VERIFY_FLAG_FUWUHAO    = 24
)
