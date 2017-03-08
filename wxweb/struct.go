package wxweb

type BaseInfo struct {
	Uin                string `json:"uin"`
	UserName           string `json:"userName,omitempty"`   // 机器人username
	WechatNick         string `json:"wechatNick,omitempty"` // 微信昵称
	ReceiveEvent       string `json:"receiveEvent,omitempty"`
	FromType           string `json:"fromType,omitempty"`
	FromUserName       string `json:"fromUserName,omitempty"`       // 群或者好友username
	FromMemberUserName string `json:"fromMemberUserName,omitempty"` // 群里用户username
	FromNickName       string `json:"fromNickName,omitempty"`       // 好友或者群里用户昵称
	FromGroupName      string `json:"fromGroupName,omitempty"`      // 群名
}

type BaseToUserInfo struct {
	ToUserName  string `json:"toUserName,omitempty"`
	ToNickName  string `json:"toNickName,omitempty"`
	ToGroupName string `json:"toGroupName,omitempty"`
}

type SendBaseInfo struct {
	WechatNick string `json:"wechatNick,omitempty"` // 微信昵称
	ChatType   string `json:"chatType,omitempty"`
	NickName   string `json:"nickName,omitempty"`
	UserName   string `json:"userName,omitempty"`
	MsgType    string `json:"msgType,omitempty"`
	Msg        string `json:"msg,omitempty"`
}

type RetResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
}

type AddFriend struct {
	SourceWechat string `json:"sourceWechat,omitempty"`
	SourceNick   string `json:"sourceNick,omitempty"`
	UserWxid     string `json:"userWxid,omitempty"`
	UserWechat   string `json:"userWechat,omitempty"`
	UserNick     string `json:"userNick,omitempty"`
	UserCity     string `json:"userCity,omitempty"`
	UserSex      int    `json:"userSex,omitempty"`
	Ticket       string `json:"-"` // for verify
}

type ReceiveMsgInfo struct {
	BaseInfo       `json:"baseInfo,omitempty"`
	BaseToUserInfo `json:"baseToUserIno,omitempty"`
	AddFriend      `json:"addFriend,omitempty"`

	MsgType        string `json:"msgType,omitempty"`
	Msg            string `json:"msg,omitempty"`
	MediaTempUrl   string `json:"mediaTempUrl,omitempty"`
	GroupMemberNum int    `json:"groupMemberNum,omitempty"`
}

type CallbackMsgInfo struct {
	RetResponse `json:"retResponse,omitempty"`
	BaseInfo    `json:"baseInfo,omitempty"`

	CallbackMsgs []SendBaseInfo `json:"msg,omitempty"`
}

type SendMsgInfo struct {
	SendMsgs []SendBaseInfo `json:"sendBaseInfo,omitempty"`
}

type SendMsgResponse struct {
	RetResponse `json:"retResponse,omitempty"`
}
