package ext

import (
	"github.com/reechou/wxrobot/wxweb"
)

type RobotSaveFriendsReq struct {
	RobotWx string             `json:"robotWx"`
	Friends []wxweb.UserFriend `json:"friends"`
}

type RobotSaveGroupsReq struct {
	RobotWx string          `json:"robotWx"`
	Groups  []wxweb.WxGroup `json:"groups"`
}

type RobotAccountResponse struct {
	Code int64       `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}
