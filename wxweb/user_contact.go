package wxweb

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/models"
)

const (
	MSG_LEN = 100
)

var (
	HostIP string
)

type UserFriend struct {
	Alias       string `json:"alias"`
	City        string `json:"city"`
	VerifyFlag  int    `json:"verifyFlag"`
	ContactFlag int    `json:"contactFlag"`
	NickName    string `json:"nickName"`
	RemarkName  string `json:"remarkName"`
	Sex         int    `json:"sex"`
	UserName    string `json:"userName"`
}

type GroupUserInfo struct {
	DisplayName string
	NickName    string
	UserName    string
}
type MsgInfo struct {
	MsgID    int
	WXMsgId  string
	NickName string
	UserName string
	Content  string
}
type MsgOffset struct {
	SliceStart int
	SliceEnd   int
	MsgIDStart int
	MsgIDEnd   int
}
type UserGroup struct {
	sync.Mutex
	ContactFlag    int
	NickName       string
	UserName       string
	MemberList     map[string]*GroupUserInfo
	NickMemberList map[string]*GroupUserInfo

	wx *WxWeb

	offset *MsgOffset
	msgs   []*MsgInfo
	msgId  int

	LastMsg     string
	LastMsgTime int64
}

func NewUserGroup(contactFlag int, nickName, userName string, wx *WxWeb) *UserGroup {
	return &UserGroup{
		ContactFlag:    contactFlag,
		NickName:       nickName,
		UserName:       userName,
		MemberList:     make(map[string]*GroupUserInfo),
		NickMemberList: make(map[string]*GroupUserInfo),
		offset: &MsgOffset{
			SliceStart: -1,
			SliceEnd:   -1,
			MsgIDStart: -1,
			MsgIDEnd:   -1,
		},
		msgs: make([]*MsgInfo, MSG_LEN),
		wx:   wx,
	}
}

func (self *UserGroup) AppendInviteMsg(msg *MsgInfo) {
	//if self.NickName == "" {
	//	logrus.Errorf("this group has no nick name.")
	//	return
	//}
	//invite := strings.Replace(msg.Content, "\"", "", -1)
	//invite = strings.Replace(invite, "邀请", ",", -1)
	//invite = strings.Replace(invite, "加入了群聊", "", -1)
	//users := strings.Split(invite, ",")
	//if len(users) != 2 {
	//	logrus.Errorf("parse invite content[%s] error.", msg.Content)
	//	return
	//}
	//inviteUsers := strings.Split(users[1], "、")
	//for _, v := range inviteUsers {
	//	has, err := self.rankRedis.HSetNX("invite_"+self.NickName, v, true)
	//	//has, err := self.rankRedis.HSetNX("invite_wx_rank", v, true)
	//	if err != nil {
	//		logrus.Errorf("hsetnx invite[%s] error: %v", v, err)
	//		continue
	//	}
	//	if !has {
	//		logrus.Debugf("has invited[%s] this man.", v)
	//		continue
	//	}
	//	//self.rankRedis.ZIncrby(self.NickName, 1, users[0])
	//	self.rankRedis.ZIncrby("wx_rank", 1, users[0])
	//}
}

func (self *UserGroup) GetInviteRank() string {
	//list := self.rankRedis.ZRevrange(self.NickName, 0, 10)
	//list := self.rankRedis.ZRevrange("wx_rank", 0, 10)
	//var usersRankInfo string
	//var userRank string
	//usersRankInfo += "邀请排行榜:\n"
	//for i := 0; i < len(list); i++ {
	//	//if i % 2 == 0 {
	//	//	userRank += "@"
	//	//}
	//	userRank += string(list[i].([]byte))
	//	if i%2 == 0 {
	//		userRank += ": "
	//	} else {
	//		userRank += "人\n"
	//		usersRankInfo += userRank
	//		userRank = ""
	//	}
	//}
	//return usersRankInfo
	return ""
}

func (self *UserGroup) AppendMsg(msg *MsgInfo) {
	self.Lock()
	defer self.Unlock()

	msg.MsgID = self.msgId

	if self.offset.SliceStart == -1 && self.offset.SliceEnd == -1 && self.offset.MsgIDStart == -1 && self.offset.MsgIDEnd == -1 {
		self.msgs[0] = msg
		self.offset.SliceStart = 0
		self.offset.SliceEnd = 1
		self.offset.MsgIDStart = msg.MsgID
		self.offset.MsgIDEnd = msg.MsgID
	} else {
		self.offset.MsgIDEnd = msg.MsgID
		self.msgs[self.offset.SliceEnd] = msg
		if self.offset.SliceEnd-self.offset.SliceStart == -1 ||
			self.offset.SliceEnd-self.offset.SliceStart == (MSG_LEN-1) ||
			self.offset.SliceEnd-self.offset.SliceStart == (1-MSG_LEN) {
			self.offset.SliceStart = (self.offset.SliceStart + 1) % MSG_LEN
			self.offset.MsgIDStart = self.msgs[self.offset.SliceStart].MsgID
		}
		self.offset.SliceEnd = (self.offset.SliceEnd + 1) % MSG_LEN
	}
	//logrus.Debugf("group[%s] add msg[%v] offset[%v]", self.UserName, msg, self.offset)

	self.msgId++
}

func (self *UserGroup) GetMsgList(msgId int) []*MsgInfo {
	if msgId >= self.offset.MsgIDEnd {
		return nil
	}
	jump := 0
	if msgId > self.offset.MsgIDStart {
		jump = msgId - self.offset.MsgIDStart
	}
	start := self.offset.SliceStart
	for i := 0; i < jump; i++ {
		start = (start + 1) % MSG_LEN
	}
	if start > self.offset.SliceEnd {
		return append(self.msgs[start:], self.msgs[:self.offset.SliceEnd]...)
	} else {
		return self.msgs[start:self.offset.SliceEnd]
	}

	return nil
}

type UserContact struct {
	sync.Mutex
	
	wx          *WxWeb
	Friends     map[string]*UserFriend
	NickFriends map[string]*UserFriend
	Groups      map[string]*UserGroup
	NickGroups  map[string]*UserGroup

	IfInviteMemberSuccess bool
}

func NewUserContact(wx *WxWeb) *UserContact {
	return &UserContact{
		wx:          wx,
		Friends:     make(map[string]*UserFriend),
		NickFriends: make(map[string]*UserFriend),
		Groups:      make(map[string]*UserGroup),
		NickGroups:  make(map[string]*UserGroup),
	}
}

func (self *UserContact) GetGroup(username string) *UserGroup {
	self.Lock()
	defer self.Unlock()
	
	return self.Groups[username]
}

func (self *UserContact) SetGroup(username string, ug *UserGroup) {
	self.Lock()
	defer self.Unlock()
	
	self.Groups[username] = ug
}

func (self *UserContact) GetNickGroup(nickname string) *UserGroup {
	self.Lock()
	defer self.Unlock()
	
	return self.NickGroups[nickname]
}

func (self *UserContact) SetNickGroup(nickname string, ug *UserGroup) {
	self.Lock()
	defer self.Unlock()
	
	self.NickGroups[nickname] = ug
}

func (self *UserContact) CreateGroups() {
	if self.wx.argv.IfCreateGroup {
		logrus.Debugf("wx[%s] create groups start.", self.wx.Session.MyNickName)
		var usernameList []string
		for _, v := range self.wx.argv.CreateGroupUsers {
			uf := self.NickFriends[v]
			if uf == nil {
				logrus.Errorf("create groups error, uf[%s] not found", v)
				return
			}
			usernameList = append(usernameList, uf.UserName)
		}
		for i := 0; i < self.wx.argv.CreateGroupNum; i++ {
			idx := self.wx.argv.CreateGroupStart + i
			res, ok := self.wx.webwxcreatechatroom(usernameList, fmt.Sprintf("%s%d", self.wx.argv.CreateGroupPrefix, idx))
			if !ok {
				logrus.Errorf("create groups error")
				return
			}
			if !CheckWebwxRetcode(res) {
				logrus.Errorf("create groups error result: %s", res)
				return
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func (self *UserContact) setIpPort(r *models.Robot) {
	r.Ip = HostIP
	r.OfPort = self.wx.cfg.Host
}

func (self *UserContact) SaveRobotFriends() {
	if self.wx.argv.IfSaveRobot {
		robot := &models.Robot{
			RobotWx: self.wx.Session.MyNickName,
		}
		has, err := models.GetRobot(robot)
		if err != nil {
			logrus.Errorf("get robot error: %v", err)
			return
		}
		if has {
			if robot.IfSaveFriend != 0 {
				logrus.Debugf("Robot[%s] has saved.", self.wx.Session.MyNickName)
				self.setIpPort(robot)
				err = models.UpdateRobotSave(robot)
				if err != nil {
					logrus.Errorf("update robot save error: %v", err)
				}
				return
			}
		} else {
			self.setIpPort(robot)
			err = models.CreateRobot(robot)
			if err != nil {
				logrus.Errorf("create robot error: %v", err)
				return
			}
		}
		var list []UserFriend
		for _, v := range self.Friends {
			_, ok := self.wx.SpecialUsers[v.UserName]
			if ok {
				continue
			}
			if v.VerifyFlag != WX_FRIEND_VERIFY_FLAG_USER {
				continue
			}
			list = append(list, *v)
			if len(list) >= 20 {
				self.wx.wxh.RobotAddFriends(self.wx.Session.MyNickName, list)
				list = nil
				time.Sleep(time.Second)
			}
		}
		if list != nil {
			self.wx.wxh.RobotAddFriends(self.wx.Session.MyNickName, list)
			list = nil
		}
		robot.IfSaveFriend = 1
		self.setIpPort(robot)
		err = models.UpdateRobotSave(robot)
		if err != nil {
			logrus.Errorf("update robot save error: %v", err)
		}
	}
}

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

func (self *UserContact) GroupMass() {
	for _, v := range self.Groups {
		if strings.Contains(v.NickName, "测试AASS") {
			if v.NickName < "测试AASS1280" {
				self.wx.Webwxsendmsg("本周给大家推荐余华的《活着》，请大家点击下方链接阅读\n\n活着（一）\nhttp://t.cn/Ric7YZ3\n\n活着（二）上\nhttp://t.cn/Ric7QQ5\n\n活着（二）下\nhttp://t.cn/Ric7nWD\n\n活着（三）\nhttp://t.cn/Ric7syt\n\n活着（四）\nhttp://t.cn/RiczPx0\n\n活着（五）上\nhttp://t.cn/RiczzNp\n\n活着（五）下\nhttp://t.cn/RiczGtz\n\n活着（六）\nhttp://t.cn/RiczcoK\n\n活着（七）\nhttp://t.cn/RiczIAH\n\n下周更新《撒哈拉的故事》，本周的大家可以收藏起来看，或者到聊天文件中找哦。", v.UserName)
				time.Sleep(2 * time.Second)
			}
		}
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

func GetHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		logrus.Errorf("GetHostName error: %v", err.Error())
		return ""
	}
	return hostName
}

func GetLocalIP(hostName string) string {
	ipAddress, err := net.ResolveIPAddr("ip", hostName)
	if err != nil {
		logrus.Errorf("GetLocalIP error: %v", err.Error())
		return ""
	}
	return ipAddress.String()
}

func replaceEmoji(oriStr string) string {
	newStr := oriStr

	if strings.Contains(oriStr, `<span class="emoji`) {
		reg, _ := regexp.Compile(`<span class="emoji emoji[a-f0-9]{5}"></span>`)
		newStr = reg.ReplaceAllStringFunc(oriStr, func(arg2 string) string {
			num := `'\U000` + arg2[len(arg2)-14:len(arg2)-9] + `'`
			emoji, err := strconv.Unquote(num)
			if err == nil {
				return emoji
			}
			return num
		})
	}

	return newStr
}

func init() {
	hostName := GetHostName()
	HostIP = GetLocalIP(hostName)
}
