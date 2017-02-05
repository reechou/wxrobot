package logic

// 变量
const (
	EMPTY     = "$empty"
	FROMGROUP = "$fromgroup"
	FROMUSER  = "$fromuser"
	FROMMSG   = "$frommsg"
)

// 函数
const (
	NOTINCLUDE      = "notinclude()"
	INCLUDE         = "include()"
	EQUAL           = "equal()"
	STATE_GROUP_NUM = "stategroupnum()"
)

const (
	CHAT_TYPE_PEOPLE = "people"
	CHAT_TYPE_GROUP  = "group"
)

const (
	MSG_TYPE_TEXT = "text"
	MSG_TYPE_IMG  = "img"
)

const (
	DO_EVENT_SENDMSG      = "sendmsg"
	DO_EVENT_VERIFY_USER  = "verifyuser"
	DO_EVENT_CALLBACK     = "callback"
	DO_EVENT_CALLBACK_RPC = "callbackrpc"
)

const (
	FUNC_EVENT_CHECK_GROUP_CHAT = "checkgroupchat"
)

const (
	EVENT_MSG_CHAN_LEN  = 1024
	WAIT_LOGIN_MAX_TIME = 360
)
