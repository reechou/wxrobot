package logic

// 变量
const (
	ALLWECHAT = "allwechat"
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

// 参数
const (
	START_WX_IfInvite        = "IfInvite"
	START_WX_IfInviteEndExit = "IfInviteEndExit"
	START_WX_InviteMsg       = "InviteMsg"
	START_WX_IfClearWx       = "IfClearWx"
	START_WX_ClearWxMsg      = "ClearWxMsg"
	START_WX_ClearWxPrefix   = "ClearWxPrefix"
)

const (
	CHAT_TYPE_PEOPLE = "people"
	CHAT_TYPE_GROUP  = "group"
)

const (
	MSG_TYPE_TEXT = "text"
	MSG_TYPE_IMG  = "img"
)

// allevent默认不处理verifyuser消息
const (
	DO_EVENT_ALL_EVENT    = "allevent"
	DO_EVENT_SENDMSG      = "sendmsg"
	DO_EVENT_VERIFY_USER  = "verifyuser"
	DO_EVENT_CALLBACK     = "callback"
	DO_EVENT_CALLBACK_RPC = "callbackrpc"
	DO_EVENT_START_WEB_WX = "startwebwx"
)

const (
	FUNC_EVENT_CHECK_GROUP_CHAT = "checkgroupchat"
)

const (
	EVENT_MSG_CHAN_LEN  = 1024
	WAIT_LOGIN_MAX_TIME = 360
)
