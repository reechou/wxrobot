package wxweb

// const about wx
const (
	MSG_TYPE_TEXT        = 1
	MSG_TYPE_INIT        = 51
	MSG_TYPE_SYSTEM      = 10000
	MSG_TYPE_VERIFY_USER = 37
)

const (
	LOGIN_WECHAT_HOST = "login.web.wechat.com"
)

const (
	WX_RET_SUCCESS = iota
)

const (
	WEBWX_SYNC_INTERVAL = 2
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
	FROM_TYPE_PEOPLE         = "people"
	FROM_TYPE_GROUP          = "group"
	RECEIVE_EVENT_MSG        = "receivemsg"
	RECEIVE_EVENT_ADD_FRIEND = "addfriend"
)

const (
	WX_VERIFY_USER_OP_ADD     = 2
	WX_VERIFY_USER_OP_CONFIRM = 3
)
