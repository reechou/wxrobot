package ext

import (
	"github.com/reechou/wxrobot/wxweb"
)

type RobotSaveFriendsReq struct {
	RobotWx string             `json:"robotWx"`
	Friends []wxweb.UserFriend `json:"friends"`
}

type RobotAccountResponse struct {
	Code int64       `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}
