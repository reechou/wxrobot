package logic

const (
	WX_RESPONSE_OK = iota
	WX_RESPONSE_ERR
)

type RobotInfo struct {
	RobotWxNick string `json:"robot"`
	RunTime     int64  `json:"runTime"`
}

type RobotFindFriendReq struct {
	WechatNick string `json:"wechatNick"`
	UserName   string `json:"username"`
	NickName   string `json:"nickname"`
}

type RobotRemarkFriendReq struct {
	WechatNick string `json:"wechatNick"`
	UserName   string `json:"username"`
	NickName   string `json:"nickname"`
	Remark     string `json:"remark"`
}

type RobotGroupTirenReq struct {
	WechatNick     string `json:"wechatNick"`
	GroupUserName  string `json:"groupUserName"`
	GroupNickName  string `json:"groupNickName"`
	MemberUserName string `json:"memberUserName"`
	MemberNickName string `json:"memberNickName"`
}

type RobotGetGroupMemberListReq struct {
	WechatNick    string `json:"wechatNick"`
	GroupUserName string `json:"groupUserName"`
	GroupNickName string `json:"groupNickName"`
}

type RobotAddFriendReq struct {
	WechatNick    string `json:"wechatNick"`
	UserName      string `json:"userName"`
	VerifyContent string `json:"verifyContent"`
}

type RobotGetLoginsReq struct {
	RobotType int `json:"robotType"`
}

type StartWxRsp struct {
	UUID       string `json:"uuid"`
	QrcodeUrl  string `json:"qrcodeUrl"`
	QrcodePath string `json:"qrcodePath"`
}

type WxResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}
