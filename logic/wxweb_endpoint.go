package logic

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/wxweb"
)

func (self *WxHttpSrv) StartWx(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "POST" {
		return nil, nil
	}

	response := WxResponse{Code: WX_RESPONSE_OK}

	uuid := self.l.StartWx()
	response.Data = uuid

	return response, nil
}

func (self *WxHttpSrv) StartWxWithArgv(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "POST" {
		return nil, nil
	}

	request := &wxweb.StartWxArgv{}
	if err := json.NewDecoder(req.Body).Decode(request); err != nil {
		logrus.Errorf("StartWxWithArgv json decode error: %v", err)
		//return nil, err
	}
	logrus.Debugf("start wx with argv[%v]", request)
	response := WxResponse{Code: WX_RESPONSE_OK}

	startRsp := self.l.StartWxWithArgv(request)
	response.Data = startRsp

	return response, nil
}

func (self *WxHttpSrv) ReceiveSendMsgs(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	request := &wxweb.SendMsgInfo{}
	if err := json.NewDecoder(req.Body).Decode(request); err != nil {
		logrus.Errorf("ReceiveSendMsgs json decode error: %v", err)
		return nil, err
	}

	response := WxResponse{Code: WX_RESPONSE_OK}

	self.l.WxSendMsgInfo(request)

	return response, nil
}

func (self *WxHttpSrv) RobotFindFriend(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	request := &RobotFindFriendReq{}
	if err := json.NewDecoder(req.Body).Decode(request); err != nil {
		logrus.Errorf("RobotFindFriend json decode error: %v", err)
		return nil, err
	}

	response := WxResponse{Code: WX_RESPONSE_OK}

	uf := self.l.RobotFindFriend(request)
	response.Data = uf

	return response, nil
}

func (self *WxHttpSrv) RobotRemarkFriend(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	request := &RobotRemarkFriendReq{}
	if err := json.NewDecoder(req.Body).Decode(request); err != nil {
		logrus.Errorf("RobotRemarkFriend json decode error: %v", err)
		return nil, err
	}

	response := WxResponse{Code: WX_RESPONSE_OK}

	ok := self.l.RobotRemarkFriend(request)
	if !ok {
		response.Code = WX_RESPONSE_ERR
	}

	return response, nil
}

func (self *WxHttpSrv) RobotGroupTiren(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	request := &RobotGroupTirenReq{}
	if err := json.NewDecoder(req.Body).Decode(request); err != nil {
		logrus.Errorf("RobotGroupTiren json decode error: %v", err)
		return nil, err
	}

	response := WxResponse{Code: WX_RESPONSE_OK}

	gui, ok := self.l.RobotGroupTiren(request)
	if !ok {
		response.Code = WX_RESPONSE_ERR
	} else {
		response.Data = gui
	}

	return response, nil
}

func (self *WxHttpSrv) ReloadEvent(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	response := WxResponse{Code: WX_RESPONSE_OK}

	self.l.eventMgr.ReloadFile()

	return response, nil
}

func (self *WxHttpSrv) AllRobots(rsp http.ResponseWriter, req *http.Request) (interface{}, error) {
	response := WxResponse{Code: WX_RESPONSE_OK}

	response.Data = self.l.GetAllRobots()

	return response, nil
}
