package wxweb

import (
	"fmt"
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

type WxGroup struct {
	NickName       string `json:"nickname"`
	UserName       string `json:"username"`
	GroupMemberNum int    `json:"groupMemberNum"`
}

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
	DisplayName string `json:"displayName"`
	NickName    string `json:"nickname"`
	UserName    string `json:"username"`
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
	ContactFlag int
	NickName    string
	UserName    string

	memberMutex    sync.Mutex
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

func (self *UserGroup) ModMember(memberList map[string]*GroupUserInfo) {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	//logrus.Debugf("old member: %v", self.MemberList)
	//logrus.Debugf("mod member: %v", memberList)
	for k, v := range memberList {
		_, ok := self.MemberList[k]
		if !ok {
			receiveMsg := &ReceiveMsgInfo{}
			receiveMsg.BaseInfo.Uin = self.wx.Session.Uin
			receiveMsg.BaseInfo.UserName = self.wx.Session.MyUserName
			receiveMsg.BaseInfo.WechatNick = self.wx.Session.MyNickName
			receiveMsg.BaseInfo.FromGroupName = self.NickName
			receiveMsg.BaseInfo.FromNickName = v.NickName
			receiveMsg.BaseInfo.FromUserName = self.UserName
			receiveMsg.BaseInfo.FromMemberUserName = v.UserName
			receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_MOD_GROUP_ADD_DETAIL
			receiveMsg.BaseInfo.FromType = FROM_TYPE_GROUP
			receiveMsg.GroupMemberNum = len(memberList)
			self.wx.wxh.ReceiveMsg(receiveMsg)
		}
	}
}

func (self *UserGroup) DelMember(username string) {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	gui := self.MemberList[username]
	if gui != nil {
		logrus.Debugf("usergroup[%s] del member[%s][%s]", self.NickName, username, gui.NickName)
		delete(self.NickMemberList, gui.NickName)
		delete(self.MemberList, username)
	}
}

func (self *UserGroup) FindMember(username, nickname string) *GroupUserInfo {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	if username != "" {
		gui := self.MemberList[username]
		if gui != nil {
			return gui
		}
	}

	return self.NickMemberList[nickname]
}

func (self *UserGroup) SetMemberList(memberList, nickMemberList map[string]*GroupUserInfo) {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	self.MemberList = memberList
	self.NickMemberList = nickMemberList
}

func (self *UserGroup) GetMemberList() map[string]*GroupUserInfo {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()
	
	return self.MemberList
}

func (self *UserGroup) GetMemberFromList(username string) *GroupUserInfo {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	return self.MemberList[username]
}

func (self *UserGroup) GetMemberFromNickList(nickname string) *GroupUserInfo {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	return self.MemberList[nickname]
}

func (self *UserGroup) GetGroupMemberLen() int {
	self.memberMutex.Lock()
	defer self.memberMutex.Unlock()

	return len(self.MemberList)
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
	groupMutex  sync.Mutex
	friendMutex sync.Mutex

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

func (self *UserContact) ChangeFriend(username, remark string) {
	self.friendMutex.Lock()
	defer self.friendMutex.Unlock()

	uf := self.Friends[username]
	if uf == nil {
		logrus.Errorf("change friend not found [%s]", username)
		return
	}
	nuf := self.NickFriends[uf.RemarkName]
	if nuf != nil {
		delete(self.NickFriends, uf.RemarkName)
		logrus.Debugf("wx[%s] delete nick friend: %s", self.wx.Session.MyNickName, uf.RemarkName)
	}
	uf.RemarkName = remark

	self.NickFriends[remark] = uf
	logrus.Debugf("wx[%s] add nick friend: %v", self.wx.Session.MyNickName, uf)
}

func (self *UserContact) FindGroup(username, nickname string) *UserGroup {
	self.groupMutex.Lock()
	defer self.groupMutex.Unlock()

	if username != "" {
		group := self.Groups[username]
		if group != nil {
			return group
		}
	}

	return self.NickGroups[nickname]
}

func (self *UserContact) FindGroupUser(groupUsername, groupNickname, memberUsername, memberNickname string) (*UserGroup, *GroupUserInfo) {
	ug := self.FindGroup(groupUsername, groupNickname)
	if ug != nil {
		//logrus.Debugf("ug: %v", ug.NickMemberList)
		return ug, ug.FindMember(memberUsername, memberNickname)
	}

	return nil, nil
}

func (self *UserContact) GetGroup(username string) *UserGroup {
	self.groupMutex.Lock()
	defer self.groupMutex.Unlock()

	return self.Groups[username]
}

func (self *UserContact) SetGroup(username string, ug *UserGroup) {
	self.groupMutex.Lock()
	defer self.groupMutex.Unlock()

	self.Groups[username] = ug
}

func (self *UserContact) GetNickGroup(nickname string) *UserGroup {
	self.groupMutex.Lock()
	defer self.groupMutex.Unlock()

	return self.NickGroups[nickname]
}

func (self *UserContact) SetNickGroup(nickname string, ug *UserGroup) {
	self.groupMutex.Lock()
	defer self.groupMutex.Unlock()

	self.NickGroups[nickname] = ug
}

func (self *UserContact) FindFriend(username, nickname string) *UserFriend {
	self.friendMutex.Lock()
	defer self.friendMutex.Unlock()

	if username != "" {
		uf := self.Friends[username]
		if uf != nil {
			return uf
		}
	}

	return self.NickFriends[nickname]
}

func (self *UserContact) GetFriend(username string) *UserFriend {
	self.friendMutex.Lock()
	defer self.friendMutex.Unlock()

	return self.Friends[username]
}

func (self *UserContact) SetFriend(username string, uf *UserFriend) {
	self.friendMutex.Lock()
	defer self.friendMutex.Unlock()

	self.Friends[username] = uf
}

func (self *UserContact) GetNickFriend(nickname string) *UserFriend {
	self.friendMutex.Lock()
	defer self.friendMutex.Unlock()

	return self.NickFriends[nickname]
}

func (self *UserContact) SetNickFriend(nickname string, uf *UserFriend) {
	self.friendMutex.Lock()
	defer self.friendMutex.Unlock()

	self.NickFriends[nickname] = uf
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

func (self *UserContact) SaveRobotGroups() {
	if self.wx.argv.IfSaveRobotGroups {
		robot := &models.Robot{
			RobotWx: self.wx.Session.MyNickName,
		}
		has, err := models.GetRobot(robot)
		if err != nil {
			logrus.Errorf("get robot error: %v", err)
			return
		}
		if has {
			if robot.IfSaveGroup != 0 {
				logrus.Debugf("Robot[%s] group has saved.", self.wx.Session.MyNickName)
				self.setIpPort(robot)
				err = models.UpdateRobotSaveGroup(robot)
				if err != nil {
					logrus.Errorf("update robot save group error: %v", err)
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
		var list []WxGroup
		for _, v := range self.Groups {
			list = append(list, WxGroup{
				NickName:       v.NickName,
				UserName:       v.UserName,
				GroupMemberNum: v.GetGroupMemberLen(),
			})
			if len(list) >= 20 {
				self.wx.wxh.RobotAddGroups(self.wx.Session.MyNickName, list)
				list = nil
				time.Sleep(time.Second)
			}
		}
		if list != nil {
			self.wx.wxh.RobotAddGroups(self.wx.Session.MyNickName, list)
			list = nil
		}
		robot.IfSaveGroup = 1
		self.setIpPort(robot)
		err = models.UpdateRobotSaveGroup(robot)
		if err != nil {
			logrus.Errorf("update robot save group error: %v", err)
		}
	}
}

func (self *UserContact) SaveRobotFriends() {
	if self.wx.argv.IfSaveRobotFriends {
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
				logrus.Debugf("Robot[%s] friend has saved.", self.wx.Session.MyNickName)
				self.setIpPort(robot)
				err = models.UpdateRobotSaveFriend(robot)
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
		err = models.UpdateRobotSaveFriend(robot)
		if err != nil {
			logrus.Errorf("update robot save friend error: %v", err)
		}
	}
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
