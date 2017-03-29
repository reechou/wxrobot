package wxweb

import (
	"fmt"
	"strings"
	"time"
	
	"github.com/Sirupsen/logrus"
)

func (self *WxWeb) webwxinit(args ...interface{}) bool {
	url := fmt.Sprintf("%s/webwxinit?pass_ticket=%s&skey=%s&r=%s",
		self.Session.BaseUri, self.Session.PassTicket, self.Session.SKey, self._unixStr())
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	res, err := self._post(url, params, true)
	if err != nil {
		return false
	}
	//logrus.Debugf("webwxinit res: %s", res)
	dataJson := JsonDecode(res)
	if dataJson == nil {
		return false
	}
	data := dataJson.(map[string]interface{})
	self.Session.User = data["User"].(map[string]interface{})
	nickName, ok := self.Session.User["NickName"]
	if ok {
		nick := nickName.(string)
		self.Session.MyNickName = nick
	}
	userName, ok := self.Session.User["UserName"]
	if ok {
		myUserName := userName.(string)
		self.Session.MyUserName = myUserName
	}
	self.Session.SyncKey = data["SyncKey"].(map[string]interface{})
	self._setsynckey()
	
	retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
	if retCode != WX_RET_SUCCESS {
		return false
	}
	chatSet := data["ChatSet"].(string)
	chats := strings.Split(chatSet, ",")
	for _, v := range chats {
		if strings.HasPrefix(v, GROUP_PREFIX) {
			ug := NewUserGroup(0, "", v, self)
			self.Contact.Groups[v] = ug
		}
	}
	logrus.Debugf("webwxinit get group num: %d", len(self.Contact.Groups))
	
	return true
}

func (self *WxWeb) webwxstatusnotify(args ...interface{}) bool {
	urlstr := fmt.Sprintf("%s/webwxstatusnotify?lang=zh_CN&pass_ticket=%s",
		self.Session.BaseUri, self.Session.PassTicket)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["Code"] = 3
	params["FromUserName"] = self.Session.User["UserName"]
	params["ToUserName"] = self.Session.User["UserName"]
	params["ClientMsgId"] = int(time.Now().Unix())
	res, err := self._post(urlstr, params, true)
	if err != nil {
		return false
	}
	
	return CheckWebwxRetcode(res)
}

func (self *WxWeb) testsynccheck(args ...interface{}) bool {
	SyncHost := []string{
		"webpush.weixin.qq.com",
		"webpush2.weixin.qq.com",
		"webpush.wechat.com",
		"webpush1.wechat.com",
		"webpush2.wechat.com",
		"webpush1.wechatapp.com",
		//"webpush.wechatapp.com"
	}
	for _, host := range SyncHost {
		self.Session.SyncHost = host
		retcode, _ := self.synccheck()
		if retcode == "0" {
			self.ifTestSyncOK = true
			logrus.Infof("test sync check host: %s", self.Session.SyncHost)
			return true
		}
	}
	return false
}
