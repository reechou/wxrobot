package wxweb

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

type AddGroupMember struct {
	sync.Mutex

	wx          *WxWeb
	uc          *UserContact
	hasAddedMap map[string]int
	groupMap    map[string]int

	addStartTime   int64
	addMemberNum   int64
	nowActiveGroup string
	nowActiveIdx   int

	stop chan struct{}
	done chan struct{}
}

func NewAddGroupMember(uc *UserContact, wx *WxWeb) *AddGroupMember {
	adm := &AddGroupMember{
		uc:           uc,
		wx:           wx,
		hasAddedMap:  make(map[string]int),
		groupMap:     make(map[string]int),
		addStartTime: time.Now().Unix(),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
	}
	if adm.wx.argv.IfSaveGroupMember {
		go adm.run()
	}

	return adm
}

func (self *AddGroupMember) Stop() {
	close(self.stop)
	<-self.done
}

func (self *AddGroupMember) run() {
	logrus.Debugf("add group member has start run.")
	for {
		select {
		case <-time.After(time.Minute):
			self.check()
		case <-self.stop:
			close(self.done)
			return
		}
	}
}

func (self *AddGroupMember) check() {
	if self.nowActiveGroup == "" {
		self.changeGroup()
	}

	now := time.Now().Unix()
	if now-self.addStartTime < self.wx.argv.AddGroupMemberCycleOfTime {
		if self.addMemberNum >= self.wx.argv.AddGroupMemberCycleOfNum {
			return
		}
	} else {
		self.addStartTime = now
		self.addMemberNum = 0
	}

	var memberList []*GroupUserInfo
	var ug *UserGroup
	for {
		if self.nowActiveGroup == "" {
			logrus.Debugf("now has none active group can add memeber.")
			return
		}
		ug = self.uc.FindGroup(self.nowActiveGroup, "")
		if ug == nil {
			logrus.Errorf("cannot found this group[%s]", self.nowActiveGroup)
			self.changeGroup()
			continue
		}
		memberList = ug.GetOriginalMemberList()
		if len(memberList) <= 10 {
			logrus.Debugf("len group: %d is too little.", len(memberList))
			self.changeGroup()
			continue
		}

		if self.nowActiveIdx >= len(memberList) {
			self.changeGroup()
			continue
		}

		break
	}

	verifyContent := fmt.Sprintf("我是[%s]的管理员", ug.NickName)
	logrus.Debugf("[group member add] start to add member: %s in group: %s", memberList[self.nowActiveIdx].NickName, ug.NickName)
	ok := self.wx.WebwxverifyuserAdd(WX_VERIFY_USER_OP_ADD, verifyContent, memberList[self.nowActiveIdx].UserName)
	if !ok {
		logrus.Errorf("webwx verify user add is not ok.")
	}
	self.addMemberNum++
	self.nowActiveIdx++

	//for i := 10; i < len(memberList); i++ {
	//	logrus.Debugf("start to add member: %s in group: %s", memberList[i].NickName, ug.NickName)
	//	ok := self.wx.WebwxverifyuserAdd(WX_VERIFY_USER_OP_ADD, "", memberList[i].UserName)
	//	if !ok {
	//		logrus.Errorf("webwx verify user add is not ok.")
	//	}
	//	time.Sleep(time.Minute)
	//}
}

func (self *AddGroupMember) changeGroup() {
	self.Lock()
	defer self.Unlock()

	if len(self.groupMap) == 0 {
		self.nowActiveGroup = ""
		return
	}

	for k, _ := range self.groupMap {
		if k == "" {
			continue
		}
		self.nowActiveGroup = k
		break
	}
	self.hasAddedMap[self.nowActiveGroup] = 1
	delete(self.groupMap, self.nowActiveGroup)
	self.nowActiveIdx = 10
	logrus.Debugf("[group member add] add group member change group to: %s", self.nowActiveGroup)
}

func (self *AddGroupMember) AddGroup(group string) {
	self.Lock()
	defer self.Unlock()

	_, ok := self.hasAddedMap[group]
	if ok {
		return
	}
	_, ok = self.groupMap[group]
	if ok {
		return
	}
	self.groupMap[group] = 1
	logrus.Debugf("[group member add] add new group: %s", group)
}
