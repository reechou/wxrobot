package logic

import (
	"strings"

	"github.com/Sirupsen/logrus"
)

type EventFilter struct {
	eventId  int
	wxm      *WxManager
	WeChat   string
	Time     string
	Event    string
	Msg      string
	From     string
	FromType string
	Do       string
	DoEvent  []DoEvent

	msgChan chan *ReceiveMsgInfo
	stop    chan struct{}
}

func (self *EventFilter) GetMsgChan() chan *ReceiveMsgInfo {
	return self.msgChan
}

func (self *EventFilter) Init(eventId int, stop chan struct{}) {
	self.msgChan = make(chan *ReceiveMsgInfo, EVENT_MSG_CHAN_LEN)
	self.stop = stop
	self.eventId = eventId

	doList := strings.Split(self.Do, "|||")
	for _, v := range doList {
		doDetail := strings.Split(v, "^")
		if len(doDetail) == 0 {
			continue
		}
		switch doDetail[0] {
		case DO_EVENT_SENDMSG:
			if len(doDetail) != 4 {
				continue
			}
			msg := &SendMsgInfo{
				WeChat:   self.WeChat,
				ChatType: doDetail[1],
				Name:     doDetail[2],
			}
			msgInfo := strings.Split(doDetail[3], ">>>")
			if len(msgInfo) != 2 {
				continue
			}
			msg.MsgType = msgInfo[0]
			msg.Msg = msgInfo[1]
			msg.Msg = strings.Replace(msg.Msg, "<br/>", "\n", -1)
			self.DoEvent = append(self.DoEvent, DoEvent{wxm: self.wxm, Type: DO_EVENT_SENDMSG, DoMsg: msg})
		case DO_EVENT_VERIFY_USER:
			self.DoEvent = append(self.DoEvent, DoEvent{wxm: self.wxm, Type: DO_EVENT_VERIFY_USER})
		case DO_EVENT_CALLBACK:
			if len(doDetail) != 2 {
				continue
			}
			self.DoEvent = append(self.DoEvent, DoEvent{wxm: self.wxm, Type: DO_EVENT_CALLBACK, DoMsg: doDetail[1]})
		case DO_EVENT_START_WEB_WX:
			if len(doDetail) != 2 && len(doDetail) != 3 {
				logrus.Errorf("filter init error: %v len(doDetail)=%d", doDetail, len(doDetail))
				continue
			}
			startWxArgv := NewStartWxArgv()
			startWxArgv.Url = doDetail[1]
			if len(doDetail) == 3 {
				argvInfo := strings.Split(doDetail[2], "&&&")
				for _, argvV := range argvInfo {
					argvEqual := strings.Split(argvV, ">>>")
					if len(argvEqual) != 2 {
						continue
					}
					switch argvEqual[0] {
					case START_WX_IfInvite:
						if argvEqual[1] == "true" {
							startWxArgv.Argv.IfInvite = true
						}
					case START_WX_IfInviteEndExit:
						if argvEqual[1] == "true" {
							startWxArgv.Argv.IfInviteEndExit = true
						}
					case START_WX_InviteMsg:
						startWxArgv.Argv.InviteMsg = argvEqual[1]
					case START_WX_IfClearWx:
						if argvEqual[1] == "true" {
							startWxArgv.Argv.IfClearWx = true
						}
					case START_WX_ClearWxMsg:
						startWxArgv.Argv.ClearWxMsg = argvEqual[1]
					case START_WX_ClearWxPrefix:
						startWxArgv.Argv.ClearWxPrefix = argvEqual[1]
					}
				}
			}
			self.DoEvent = append(self.DoEvent, DoEvent{wxm: self.wxm, Type: DO_EVENT_START_WEB_WX, DoMsg: startWxArgv})
		}
	}
	go self.Run()
}

func (self *EventFilter) Run() {
	logrus.Debugf("filter wechat[%s] Event[%s] Do[%s] start run.", self.WeChat, self.Event, self.Do)
	for {
		select {
		case msg := <-self.msgChan:
			//logrus.Debugf("filter msg: %v", msg.msg)
			if self.Event != msg.msg.BaseInfo.ReceiveEvent {
				if self.Event != DO_EVENT_ALL_EVENT {
					continue
				}
			}
			if self.FromType != "" {
				if self.FromType != msg.msg.BaseInfo.FromType {
					continue
				}
			}
			if self.Msg != "" {
				if !ExecCheckFunc(self.Msg, msg.msg.Msg) {
					continue
				}
			}
			logrus.Debugf("filter[%d] msg: %v", self.eventId, msg.msg)
			if self.From != "" {
				if msg.msg.BaseInfo.FromType == CHAT_TYPE_GROUP {
					if !ExecCheckFunc(self.From, msg.msg.BaseInfo.FromGroupName) {
						continue
					}
				}
			}
			for _, v := range self.DoEvent {
				v.Do(msg)
			}
		case <-self.stop:
			logrus.Infof("filter do[%s] stopped")
			return
		}
	}
}
