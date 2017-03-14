package wxweb

import (
	"time"
	"math/rand"
	"strings"
	
	"github.com/Sirupsen/logrus"
)

func (self *UserContact) ClearWx() {
	if self.wx.argv.IfClearWx {
		logrus.Debugf("clear wx[%s] start.", self.wx.Session.MyNickName)
		for _, v := range self.Friends {
			_, ok := self.wx.SpecialUsers[v.UserName]
			if ok {
				continue
			}
			if v.VerifyFlag != WX_FRIEND_VERIFY_FLAG_USER {
				continue
			}
			self.wx.Webwxsendmsg(self.wx.argv.ClearWxMsg, v.UserName)
			time.Sleep(3 * time.Second)
		}
		logrus.Debugf("clear wx[%s] end.", self.wx.Session.MyNickName)
	}
}

func (self *UserContact) InviteMembersPic() {
	if self.wx.cfg.IfInvite {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for _, v := range self.Friends {
			_, ok := self.wx.SpecialUsers[v.UserName]
			if ok {
				continue
			}
			mediaId, ok := self.wx.Webwxuploadmedia(v.UserName, self.wx.cfg.UploadFile)
			if ok {
				self.wx.Webwxsendmsgimg(v.UserName, mediaId)
			}
			time.Sleep(time.Duration(r.Intn(17)+11) * time.Second)
		}
		self.IfInviteMemberSuccess = true
		logrus.Infof("[%s] invite members success.", self.wx.Session.MyNickName)
	}
}

func (self *UserContact) InviteMembers() {
	if self.wx.argv.IfInvite {
		inviteMsg := self.wx.argv.InviteMsg
		if inviteMsg == "" {
			inviteMsg = self.wx.cfg.InviteMsg
		}
		var groupUserName string
		for _, v := range self.Groups {
			if strings.Contains(v.NickName, "网购特卖") {
				groupUserName = v.UserName
				break
			}
		}
		if groupUserName != "" {
			inviteNum := 0
			var friends []*UserFriend
			var otherFriends []*UserFriend
			for _, v := range self.Friends {
				if v.Sex == WX_GIRL {
					friends = append(friends, v)
				} else {
					otherFriends = append(otherFriends, v)
				}
			}
			friends = append(friends, otherFriends...)
			var memberList []string
			for _, v := range friends {
				_, ok := self.wx.SpecialUsers[v.UserName]
				if ok {
					continue
				}
				if v.VerifyFlag != WX_FRIEND_VERIFY_FLAG_USER {
					continue
				}
				memberList = append(memberList, v.UserName)
				if len(memberList) >= 9 {
					data, ok := self.wx.WebwxupdatechatroomInvitemember(groupUserName, memberList)
					if ok {
						dataJson := JsonDecode(data)
						if dataJson != nil {
							dataMap := dataJson.(map[string]interface{})
							retCode := dataMap["BaseResponse"].(map[string]interface{})["Ret"].(int)
							if retCode == -34 {
								logrus.Errorf("wx[%s] invite member get -34 error, maybe sleep some minute", self.wx.Session.MyNickName)
								time.Sleep(17 * time.Minute)
							} else {
								for _, v2 := range memberList {
									self.wx.Webwxsendmsg(inviteMsg, v2)
									time.Sleep(7 * time.Second)
								}
							}
						}
					}
					inviteNum += 10
					// clear
					memberList = nil
					time.Sleep(8 * time.Second)
					if inviteNum >= 200 {
						time.Sleep(2 * time.Minute)
						inviteNum = 0
					}
				}
			}
			if memberList != nil {
				time.Sleep(8 * time.Second)
				self.wx.WebwxupdatechatroomInvitemember(groupUserName, memberList)
				for _, v2 := range memberList {
					self.wx.Webwxsendmsg(inviteMsg, v2)
					time.Sleep(5 * time.Second)
				}
				// clear
				memberList = nil
			}
		} else {
			logrus.Errorf("check group not found.")
		}
		self.IfInviteMemberSuccess = true
		logrus.Infof("[%s] invite members success.", self.wx.Session.MyNickName)
	}
	if self.wx.argv.IfInviteEndExit {
		self.wx.Stop()
	}
}

func (self *UserContact) PrintGroupInfo() {
	allGroupNum := 0
	cfNum := 0
	members := make(map[string]int)
	for _, v := range self.Groups {
		logrus.Debugf("[*] 群: %s", v.NickName)
		allGroupNum++
		for _, v2 := range v.MemberList {
			// check verify user
			//self.wx.Webwxverifyuser(WX_VERIFY_USER_OP_ADD, "你好", "", v2.UserName)
			//time.Sleep(10 * time.Second)
			
			_, ok := members[v2.UserName]
			if ok {
				cfNum++
				continue
			}
			members[v2.UserName] = 1
		}
		
		// test
		//if v.NickName == "xxxx" {
		//	logrus.Debugf("xxxx: %v", v)
		//	for _, v := range v.MemberList {
		//		logrus.Debugf("\tmember: %v", v)
		//	}
		//}
	}
	logrus.Info("[*] 群组数:", allGroupNum)
	logrus.Info("[*] 好友数:", len(self.Friends))
	//logrus.Info("[*] 去重群成员总数:", len(members), cfNum)
}
