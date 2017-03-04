package logic

import (
	"fmt"
	"strings"
	"sync"
	"io"
	"crypto/md5"
	"os"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/wxweb"
	"github.com/reechou/wxrobot/config"
)

type WxManager struct {
	sync.Mutex
	wxs map[string]*wxweb.WxWeb
	cfg *config.Config
}

func NewWxManager(cfg *config.Config) *WxManager {
	wm := &WxManager{
		wxs: make(map[string]*wxweb.WxWeb),
		cfg: cfg,
	}
	return wm
}

func (self *WxManager) RegisterWx(wx *wxweb.WxWeb) {
	self.Lock()
	defer self.Unlock()

	nickName, ok := wx.Session.User["NickName"]
	if ok {
		nick := nickName.(string)
		self.wxs[nick] = wx
		logrus.Infof("wx manager register wx[%s] success.", nick)
	}
}

func (self *WxManager) UnregisterWx(wx *wxweb.WxWeb) {
	self.Lock()
	defer self.Unlock()

	nickName, ok := wx.Session.User["NickName"]
	if ok {
		nick := nickName.(string)
		_, ok := self.wxs[nick]
		if ok {
			delete(self.wxs, nick)
			logrus.Infof("wx manager unregister wx[%s] success.", nick)
		}
	}
}

func (self *WxManager) SendMsg(msg *SendMsgInfo, msgStr string) {
	wx := self.wxs[msg.WeChat]
	if wx == nil {
		logrus.Errorf("send msg unknown this wechat[%s].", msg.WeChat)
		return
	}
	switch msg.ChatType {
	case CHAT_TYPE_PEOPLE:
		var userName string
		if msg.UserName != "" {
			userName = msg.UserName
			uf := wx.Contact.Friends[userName]
			if uf == nil {
				uf := wx.Contact.NickFriends[msg.Name]
				if uf == nil {
					logrus.Errorf("unkown this friend[%s]", msg.Name)
					return
				}
				userName = uf.UserName
				logrus.Debugf("send msg to people find username[%s] from name[%s]", userName, msg.Name)
			} else {
				logrus.Debugf("send msg to people find username[%s] from request", userName)
			}
		} else {
			uf := wx.Contact.NickFriends[msg.Name]
			if uf == nil {
				logrus.Errorf("unkown this friend[%s]", msg.Name)
				return
			}
			userName = uf.UserName
		}
		if msg.MsgType == MSG_TYPE_TEXT {
			wx.Webwxsendmsg(msgStr, userName)
		} else if msg.MsgType == MSG_TYPE_IMG {
			self.sendImg(userName, msgStr, wx)
		}
	case CHAT_TYPE_GROUP:
		var userName string
		if msg.UserName != "" {
			userName = msg.UserName
			group := wx.Contact.GetGroup(userName)
			if group == nil {
				group := wx.Contact.GetNickGroup(msg.Name)
				if group == nil {
					logrus.Errorf("unkown this group[%s]", msg.Name)
					return
				}
				userName = group.UserName
				logrus.Debugf("send msg to group find username[%s] from name[%s]", userName, msg.Name)
			} else {
				logrus.Debugf("send msg to group find username[%s] from request", userName)
			}
		} else {
			group := wx.Contact.GetNickGroup(msg.Name)
			if group == nil {
				logrus.Errorf("unkown this group[%s]", msg.Name)
				return
			}
			userName = group.UserName
		}
		if msg.MsgType == MSG_TYPE_TEXT {
			wx.Webwxsendmsg(msgStr, userName)
		} else if msg.MsgType == MSG_TYPE_IMG {
			self.sendImg(userName, msgStr, wx)
		}
	}
}

func (self *WxManager) sendImg(userName, imgMsg string, wx *wxweb.WxWeb) {
	if strings.HasPrefix(imgMsg, "http") {
		logrus.Debugf("send img[%s] to username[%s]", imgMsg, userName)
		imgPath := fmt.Sprintf("%s/%s.jpg", self.cfg.TempPicDir, fmt.Sprintf("%x", md5.Sum([]byte(imgMsg))))
		if !PathExist(imgPath) {
			logrus.Debugf("img[%s] not exist", imgPath)
			res, err := http.Get(imgMsg)
			if err != nil {
				logrus.Errorf("http get img[%s] error: %v", imgMsg, err)
				return
			}
			out, err := os.Create(imgPath)
			if err != nil {
				logrus.Errorf("os create img[%s] error: %v", imgPath, err)
				return
			}
			_, err = io.Copy(out, res.Body)
			if err != nil {
				logrus.Errorf("io copy img[%s] error: %v", imgPath, err)
				return
			}
		}
		mediaId, ok := wx.Webwxuploadmedia(userName, imgPath)
		if ok {
			wx.Webwxsendmsgimg(userName, mediaId)
		}
	} else {
		mediaId, ok := wx.Webwxuploadmedia(userName, imgMsg)
		if ok {
			wx.Webwxsendmsgimg(userName, mediaId)
		}
	}
}

func (self *WxManager) SendImgMsg(msg *SendImgInfo) {
	wx := self.wxs[msg.WeChat]
	if wx == nil {
		logrus.Errorf("send img msg unknown this wechat[%s].", msg.WeChat)
		return
	}
	mediaId, ok := wx.Webwxuploadmedia(msg.UserName, msg.ImgPath)
	if ok {
		wx.Webwxsendmsgimg(msg.UserName, mediaId)
	}
}

func (self *WxManager) VerifyUser(msg *wxweb.ReceiveMsgInfo) bool {
	wx := self.wxs[msg.BaseInfo.WechatNick]
	if wx == nil {
		logrus.Errorf("unknown this wechat[%s].", msg.BaseInfo.WechatNick)
		return false
	}
	realName, ok := wx.Webwxverifyuser(wxweb.WX_VERIFY_USER_OP_CONFIRM, "", msg.AddFriend.Ticket, msg.BaseInfo.FromUserName, msg.BaseInfo.FromNickName)
	if ok {
		logrus.Infof("verify user[%s] success, and remark name[%s]", msg.BaseInfo.FromNickName, realName)
		msg.BaseInfo.FromNickName = realName
		msg.AddFriend.UserNick = realName
	} else {
		logrus.Infof("verify user[%s] error.", msg.BaseInfo.FromNickName)
	}
	return ok
}

func (self *WxManager) CheckGroup() {

}

func (self *WxManager) StateGroupNum(wechat, g string) string {
	wx := self.wxs[wechat]
	if wx == nil {
		logrus.Errorf("unknown this wechat[%s].", wechat)
		return ""
	}
	g = strings.Replace(g, "\n", "", -1)
	result := "[奸笑][奸笑][奸笑]【%s | 群】群总数-%d 去重群成员数-%d 重复成员数-%d"
	allGroupNum := 0
	cfNum := 0
	members := make(map[string]int)
	for _, v := range wx.Contact.Groups {
		if !ExecCheckFunc(g, v.NickName) {
			continue
		}
		allGroupNum++
		for _, v2 := range v.MemberList {
			_, ok := members[v2.UserName]
			if ok {
				cfNum++
				continue
			}
			members[v2.UserName] = 1
		}
	}
	g = ExecGetArgvFunc(g)
	return fmt.Sprintf(result, g, allGroupNum, len(members), cfNum)
}

func (self *WxManager) CheckGroupChat(info *CheckGroupChatInfo) {

}

func PathExist(p string) bool {
	_, err := os.Stat(p)
	return err == nil || os.IsExist(err)
}
