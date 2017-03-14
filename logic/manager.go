package logic

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/config"
	"github.com/reechou/wxrobot/wxweb"
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

func (self *WxManager) SendMsg(msg *SendMsgInfo, msgStr string) bool {
	wx := self.wxs[msg.WeChat]
	if wx == nil {
		logrus.Errorf("send msg unknown this wechat[%s].", msg.WeChat)
		return false
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
					return false
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
				return false
			}
			userName = uf.UserName
		}
		if msg.MsgType == MSG_TYPE_TEXT {
			return wx.Webwxsendmsg(msgStr, userName)
		} else if msg.MsgType == MSG_TYPE_IMG {
			return self.sendImg(userName, msgStr, wx)
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
					return false
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
				return false
			}
			userName = group.UserName
		}
		if msg.MsgType == MSG_TYPE_TEXT {
			return wx.Webwxsendmsg(msgStr, userName)
		} else if msg.MsgType == MSG_TYPE_IMG {
			return self.sendImg(userName, msgStr, wx)
		}
	}
	return false
}

func (self *WxManager) sendImg(userName, imgMsg string, wx *wxweb.WxWeb) bool {
	if strings.HasPrefix(imgMsg, "http") {
		logrus.Debugf("send img[%s] to username[%s]", imgMsg, userName)
		imgPath := fmt.Sprintf("%s/%s.jpg", self.cfg.TempPicDir, fmt.Sprintf("%x", md5.Sum([]byte(imgMsg))))
		if !PathExist(imgPath) {
			logrus.Debugf("img[%s] not exist", imgPath)
			res, err := http.Get(imgMsg)
			if err != nil {
				logrus.Errorf("http get img[%s] error: %v", imgMsg, err)
				return false
			}
			out, err := os.Create(imgPath)
			if err != nil {
				logrus.Errorf("os create img[%s] error: %v", imgPath, err)
				return false
			}
			_, err = io.Copy(out, res.Body)
			if err != nil {
				logrus.Errorf("io copy img[%s] error: %v", imgPath, err)
				return false
			}
		}
		mediaId, ok := wx.Webwxuploadmedia(userName, imgPath)
		if ok {
			return wx.Webwxsendmsgimg(userName, mediaId)
		}
	} else {
		mediaId, ok := wx.Webwxuploadmedia(userName, imgMsg)
		if ok {
			return wx.Webwxsendmsgimg(userName, mediaId)
		}
	}
	return false
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

func (self *WxManager) FindFriend(info *RobotFindFriendReq) *wxweb.UserFriend {
	wx := self.wxs[info.WechatNick]
	if wx == nil {
		logrus.Errorf("find friend unknown this wechat[%s].", info.WechatNick)
		return nil
	}
	return wx.Contact.FindFriend(info.UserName, info.NickName)
}

func (self *WxManager) RemarkFriend(info *RobotRemarkFriendReq) bool {
	wx := self.wxs[info.WechatNick]
	if wx == nil {
		logrus.Errorf("remark friend unknown this wechat[%s].", info.WechatNick)
		return false
	}

	uf := wx.Contact.FindFriend(info.UserName, info.NickName)
	if uf == nil {
		logrus.Errorf("cannot found this friend[%v]", info)
		return false
	}

	ok := wx.WebwxOplog(uf.UserName, info.Remark)
	if ok {
		wx.Contact.ChangeFriend(uf.UserName, info.Remark)
	}

	return ok
}

func (self *WxManager) GroupTiren(info *RobotGroupTirenReq) (*wxweb.GroupUserInfo, bool) {
	wx := self.wxs[info.WechatNick]
	if wx == nil {
		logrus.Errorf("group tiren unknown this wechat[%s].", info.WechatNick)
		return nil, false
	}
	ug, gui := wx.Contact.FindGroupUser(info.GroupUserName, info.GroupNickName, info.MemberUserName, info.MemberNickName)
	if gui != nil {
		ok := wx.DelMemberWebwxupdatechatroom(ug.UserName, gui.UserName)
		if ok {
			ug.DelMember(gui.UserName)
		}
		return gui, ok
	}
	logrus.Errorf("wx[%s] find group user none: %v", wx.Session.MyNickName, info)
	return nil, false
}

func (self *WxManager) LoginRobots() []RobotInfo {
	self.Lock()
	defer self.Unlock()

	var list []RobotInfo
	for _, v := range self.wxs {
		if v.IfLogin() {
			list = append(list, RobotInfo{
				RobotWxNick: v.RobotWxNick(),
				RunTime:     time.Now().Unix() - v.StartTime(),
			})
		}
	}
	return list
}

func PathExist(p string) bool {
	_, err := os.Stat(p)
	return err == nil || os.IsExist(err)
}
