package logic

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	jsonrpc "github.com/gorilla/rpc/json"
	"github.com/reechou/wxrobot/wxweb"
)

type DoEvent struct {
	wxm   *WxManager
	Type  string
	DoMsg interface{}

	client *http.Client
}

func (self *DoEvent) Do(rMsg *ReceiveMsgInfo) {
	switch self.Type {
	case DO_EVENT_SENDMSG:
		msg, ok := self.DoMsg.(*SendMsgInfo)
		if ok {
			msgCopy := &SendMsgInfo{
				WeChat:   msg.WeChat,
				ChatType: msg.ChatType,
				Name:     msg.Name,
				UserName: msg.UserName,
				MsgType:  msg.MsgType,
				Msg:      msg.Msg,
			}
			if msgCopy.Name == "$from" {
				msgCopy.Name = rMsg.msg.BaseInfo.FromNickName
			}
			msgCopy.UserName = rMsg.msg.BaseInfo.FromUserName
			msgResult := self.changeString(msgCopy, rMsg)
			self.wxm.SendMsg(msgCopy, msgResult)
		} else {
			logrus.Errorf("translate to SendMsgInfo error.")
		}
	case DO_EVENT_VERIFY_USER:
		self.wxm.VerifyUser(rMsg.msg)
	case DO_EVENT_CALLBACK:
		self.call(rMsg)
	case DO_EVENT_CALLBACK_RPC:
		self.callrpc(rMsg)
	case DO_EVENT_START_WEB_WX:
		self.startwebwx(rMsg)
	}
}

func (self *DoEvent) startwebwx(rMsg *ReceiveMsgInfo) {
	argv := self.DoMsg.(*StartWxArgv)
	if argv == nil {
		logrus.Errorf("do event[startwebwx] argv == nil, please change config.")
		return
	}
	if self.client == nil {
		self.client = &http.Client{}
	}
	
	reqBytes, err := json.Marshal(argv.Argv)
	if err != nil {
		logrus.Errorf("do event[startwebwx] json encode error: %v", err)
		return
	}
	req, err := http.NewRequest("POST", argv.Url, bytes.NewBuffer(reqBytes))
	if err != nil {
		logrus.Errorf("do event[startwebwx] http new request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := self.client.Do(req)
	if err != nil {
		logrus.Errorf("do event[startwebwx] http do request error: %v", err)
		return
	}
	defer resp.Body.Close()
	rspBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("do event[startwebwx] ioutil ReadAll error: %v", err)
		return
	}
	var callbackMsg WxResponse
	err = json.Unmarshal(rspBody, &callbackMsg)
	if err != nil {
		logrus.Errorf("do event[startwebwx] json decode error: %v", err)
		return
	}
	if callbackMsg.Code != WX_RESPONSE_OK {
		logrus.Errorf("do event[startwebwx] ret error: %d %s", callbackMsg.Code, callbackMsg.Msg)
		return
	}
	
}

func (self *DoEvent) callrpc(rMsg *ReceiveMsgInfo) {
	if self.client == nil {
		self.client = &http.Client{}
	}
	url := self.DoMsg.(string)
	if url == "" {
		logrus.Errorf("do event[callrpc] url == nil, please change config.")
		return
	}

	message, err := jsonrpc.EncodeClientRequest("robot.callback", rMsg.msg)
	if err != nil {
		logrus.Errorf("[callrpc] json encode client request error: %v", err)
		return
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(message))
	if err != nil {
		logrus.Errorf("[callrpc] http new request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := self.client.Do(req)
	if err != nil {
		logrus.Errorf("[callrpc] http do request error: %v", err)
		return
	}
	defer resp.Body.Close()
	var callbackMsg wxweb.CallbackMsgInfo
	err = jsonrpc.DecodeClientResponse(resp.Body, &callbackMsg)
	if err != nil {
		logrus.Errorf("[callrpc] json decode client response error: %v", err)
		return
	}
	if callbackMsg.RetResponse.Code != 0 {
		logrus.Errorf("do event[callback] ret error: %d %s", callbackMsg.RetResponse.Code, callbackMsg.RetResponse.Msg)
		return
	}
	for _, v := range callbackMsg.CallbackMsgs {
		if v.Msg != "" {
			msg := &SendMsgInfo{
				WeChat:   v.WechatNick,
				ChatType: v.ChatType,
				Name:     v.NickName,
				UserName: v.UserName,
				MsgType:  v.MsgType,
				Msg:      v.Msg,
			}
			self.wxm.SendMsg(msg, msg.Msg)
		}
	}
}

func (self *DoEvent) call(rMsg *ReceiveMsgInfo) {
	if self.client == nil {
		self.client = &http.Client{}
	}
	url := self.DoMsg.(string)
	if url == "" {
		logrus.Errorf("do event[callback] url == nil, please change config.")
		return
	}

	reqBytes, err := json.Marshal(rMsg.msg)
	if err != nil {
		logrus.Errorf("do event[callback] json encode error: %v", err)
		return
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		logrus.Errorf("do event[callback] http new request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := self.client.Do(req)
	if err != nil {
		logrus.Errorf("do event[callback] http do request error: %v", err)
		return
	}
	defer resp.Body.Close()
	rspBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("do event[callback] ioutil ReadAll error: %v", err)
		return
	}
	//logrus.Debug(string(reqBytes), " ", url, " ", string(rspBody))
	var callbackMsg wxweb.CallbackMsgInfo
	err = json.Unmarshal(rspBody, &callbackMsg)
	if err != nil {
		logrus.Errorf("do event[callback] json decode error: %v", err)
		return
	}
	if callbackMsg.RetResponse.Code != 0 {
		logrus.Errorf("do event[callback] ret error: %d %s", callbackMsg.RetResponse.Code, callbackMsg.RetResponse.Msg)
		return
	}
	//logrus.Debugf("call back rsp: %v", callbackMsg)
	for _, v := range callbackMsg.CallbackMsgs {
		if v.Msg != "" {
			msg := &SendMsgInfo{
				WeChat:   v.WechatNick,
				ChatType: v.ChatType,
				Name:     v.NickName,
				UserName: v.UserName,
				MsgType:  v.MsgType,
				Msg:      v.Msg,
			}
			self.wxm.SendMsg(msg, msg.Msg)
		}
	}
}

func (self *DoEvent) changeString(sm *SendMsgInfo, rm *ReceiveMsgInfo) string {
	result := sm.Msg
	result = strings.Replace(result, "\\n", "\n", -1)
	if strings.Contains(sm.Msg, FROMGROUP) {
		result = strings.Replace(result, FROMGROUP, rm.msg.BaseInfo.FromGroupName, -1)
	}
	if strings.Contains(sm.Msg, FROMUSER) {
		result = strings.Replace(result, FROMUSER, rm.msg.BaseInfo.FromNickName, -1)
	}
	if strings.Contains(sm.Msg, FROMMSG) {
		result = strings.Replace(result, FROMMSG, rm.msg.Msg, -1)
	}

	if strings.HasPrefix(sm.Msg, STATE_GROUP_NUM) {
		result = strings.Replace(result, STATE_GROUP_NUM, "", -1)
		result = self.wxm.StateGroupNum(sm.WeChat, result)
	}

	return result
}
