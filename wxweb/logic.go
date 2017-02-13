package wxweb

import (
	//"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
)

type AddFriendContent struct {
	Msg            xml.Name `xml:"msg"`
	SourceUsername string   `xml:"sourceusername,attr"`
	SourceNickname string   `xml:"sourcenickname,attr"`
	FromUsername   string   `xml:"fromusername,attr"`
	FromNickname   string   `xml:"fromnickname,attr"`
}

func (self *WxWeb) handleMsg(r interface{}) {
	if r == nil {
		return
	}
	msgSource := r.(map[string]interface{})
	if msgSource == nil {
		return
	}

	modContactList := msgSource["ModContactList"]
	if modContactList != nil {
		contactList := modContactList.([]interface{})
		if contactList != nil {
			for _, v := range contactList {
				modContact := v.(map[string]interface{})
				if modContact == nil {
					continue
				}
				userName := modContact["UserName"].(string)
				if strings.HasPrefix(userName, GROUP_PREFIX) {
					// 群成员变化
					groupContactFlag := modContact["ContactFlag"].(int)
					groupNickName := modContact["NickName"].(string)
					group := self.Contact.Groups[userName]
					if group == nil {
						group = NewUserGroup(groupContactFlag, groupNickName, userName, self)
					} else {
						group.ContactFlag = groupContactFlag
						group.NickName = groupNickName
					}
					memberList := modContact["MemberList"].([]interface{})
					memberListMap := make(map[string]*GroupUserInfo)
					for _, v2 := range memberList {
						member := v2.(map[string]interface{})
						if member == nil {
							logrus.Errorf("handlemsg get member[%v] error", v2)
							continue
						}
						displayName := member["DisplayName"].(string)
						nickName := member["NickName"].(string)
						userName := member["UserName"].(string)
						gui := &GroupUserInfo{
							DisplayName: displayName,
							NickName:    nickName,
							UserName:    userName,
						}
						memberListMap[userName] = gui
					}
					group.MemberList = memberListMap
					self.Contact.Groups[userName] = group
					self.Contact.NickGroups[groupNickName] = group
				} else {
					// 新好友
					userContactFlag := modContact["ContactFlag"].(int)
					userVerifyFlag := modContact["VerifyFlag"].(int)
					userNickName := modContact["NickName"].(string)
					alias := modContact["Alias"].(string)
					city := modContact["City"].(string)
					sex := modContact["Sex"].(int)
					user := self.Contact.Friends[userName]
					if user == nil {
						uf := &UserFriend{
							Alias:       alias,
							City:        city,
							VerifyFlag:  userVerifyFlag,
							ContactFlag: userContactFlag,
							NickName:    userNickName,
							Sex:         sex,
							UserName:    userName,
						}
						self.Contact.Friends[userName] = uf
						self.Contact.NickFriends[userNickName] = uf
					}
				}
			}
		}
	}

	addMsgList := msgSource["AddMsgList"]
	if addMsgList == nil {
		return
	}
	msgList := addMsgList.([]interface{})
	if msgList == nil {
		return
	}
	for _, v := range msgList {
		msg := v.(map[string]interface{})
		if msg == nil {
			continue
		}
		msgType := msg["MsgType"].(int)
		fromUserName := msg["FromUserName"].(string)
		content := msg["Content"].(string)
		content = strings.Replace(content, "&lt;", "<", -1)
		content = strings.Replace(content, "&gt;", ">", -1)
		content = strings.Replace(content, " ", " ", 1)
		msgid := msg["MsgId"].(string)
		receiveMsg := &ReceiveMsgInfo{}
		receiveMsg.BaseInfo.Uin = self.uin
		receiveMsg.BaseInfo.WechatNick = self.MyNickName
		receiveMsg.BaseInfo.FromUserName = fromUserName
		// 文本消息
		if msgType == MSG_TYPE_TEXT {
			//logrus.Debugf("text msg: %s", content)
			if strings.Contains(content, MSG_MEDIA_KEYWORD) {
				continue
			}
			if fromUserName[:2] == GROUP_PREFIX {
				contentSlice := strings.Split(content, ":<br/>")
				people := contentSlice[0]
				content = contentSlice[1]
				group := self.Contact.Groups[fromUserName]
				if group == nil {
					logrus.Errorf("cannot found the group[%s]", fromUserName)
					continue
				}
				sendPeople := group.MemberList[people]
				if sendPeople == nil {
					continue
				}
				msg := &MsgInfo{
					WXMsgId:  msgid,
					NickName: sendPeople.NickName,
					UserName: sendPeople.UserName,
					Content:  content,
				}
				group.AppendMsg(msg)

				// 读取消息
				//self.webwxstatusnotifyMsgRead(fromUserName)

				receiveMsg.BaseInfo.FromGroupName = group.NickName
				receiveMsg.BaseInfo.FromNickName = sendPeople.NickName
				receiveMsg.BaseInfo.FromType = FROM_TYPE_GROUP
			} else {
				uf, ok := self.Contact.Friends[receiveMsg.BaseInfo.FromUserName]
				if ok {
					receiveMsg.BaseInfo.FromNickName = uf.NickName
				}
				receiveMsg.BaseInfo.FromType = FROM_TYPE_PEOPLE
				self.webwxstatusnotifyMsgRead(receiveMsg.BaseInfo.FromUserName)
			}
			receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_MSG
			receiveMsg.Msg = content
		} else if msgType == MSG_TYPE_INIT {
			//logrus.Debug("[*] 成功截获微信初始化消息")
		} else if msgType == MSG_TYPE_SYSTEM {
			logrus.Debugf("系统消息: %s", content)
			//if strings.Contains(content, "邀请") {
			//	group := self.Contact.Groups[fromUserName]
			//	if group == nil {
			//		continue
			//	}
			//	//group.AppendInviteMsg(&MsgInfo{WXMsgId: msgid, Content: content})
			//}

			// 系统消息不是好友
			if strings.Contains(content, WX_SYSTEM_NOT_FRIEND) {
				if self.argv.IfClearWx {
					prefix := self.argv.ClearWxPrefix
					if prefix == "" {
						prefix = CLEAR_WX_PREFIX_DEFAULT
					}
					user := self.Contact.Friends[fromUserName]
					userNick := ""
					if user != nil {
						userNick = user.NickName
					}
					self.WebwxOplog(fromUserName, fmt.Sprintf("%s %s", prefix, userNick))
				}
			}
		} else if msgType == MSG_TYPE_VERIFY_USER {
			recommendInfo := msg["RecommendInfo"]
			if recommendInfo == nil {
				logrus.Errorf("recommendInfo == nil")
				return
			}
			rInfo := recommendInfo.(map[string]interface{})
			if rInfo == nil {
				logrus.Errorf("rInfo == nil")
				return
			}
			//wechat := rInfo["Alias"].(string)
			ticket := rInfo["Ticket"].(string)
			userName := rInfo["UserName"].(string)
			nickName := rInfo["NickName"].(string)

			receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_ADD_FRIEND
			receiveMsg.BaseInfo.FromNickName = nickName
			receiveMsg.BaseInfo.FromUserName = userName
			receiveMsg.BaseInfo.FromType = FROM_TYPE_PEOPLE
			receiveMsg.AddFriend.Ticket = ticket

			logrus.Debugf("addfriend conteng: %s", content)
			//var addFriendContent AddFriendContent
			//err := xml.Unmarshal([]byte(content), &addFriendContent)
			//if err != nil {
			//	logrus.Errorf("add friend parse content error: %v", err)
			//} else {
			//	receiveMsg.AddFriend.SourceWechat = addFriendContent.SourceUsername
			//	receiveMsg.AddFriend.SourceNick = addFriendContent.SourceNickname
			//	receiveMsg.AddFriend.UserWechat = addFriendContent.FromUsername
			//	receiveMsg.AddFriend.UserNick = addFriendContent.FromNickname
			//}
			reg := regexp.MustCompile(`alias(.*?)=(.*?)\"(.*?)\"`)
			alias := reg.FindString(string(content))
			alias = strings.Replace(alias, "\"", "", -1)
			alias = strings.Replace(alias, "alias=", "", -1)

			reg = regexp.MustCompile(`fromusername(.*?)=(.*?)\"(.*?)\"`)
			fromusername := reg.FindString(string(content))
			fromusername = strings.Replace(fromusername, "\"", "", -1)
			fromusername = strings.Replace(fromusername, "fromusername=", "", -1)

			reg = regexp.MustCompile(`fromnickname(.*?)=(.*?)\"(.*?)\"`)
			fromnickname := reg.FindString(string(content))
			fromnickname = strings.Replace(fromnickname, "\"", "", -1)
			fromnickname = strings.Replace(fromnickname, "fromnickname=", "", -1)

			reg = regexp.MustCompile(`sourceusername(.*?)=(.*?)\"(.*?)\"`)
			sourceusername := reg.FindString(string(content))
			sourceusername = strings.Replace(sourceusername, "\"", "", -1)
			sourceusername = strings.Replace(sourceusername, "sourceusername=", "", -1)

			reg = regexp.MustCompile(`sourcenickname(.*?)=(.*?)\"(.*?)\"`)
			sourcenickname := reg.FindString(string(content))
			sourcenickname = strings.Replace(sourcenickname, "\"", "", -1)
			sourcenickname = strings.Replace(sourcenickname, "sourcenickname=", "", -1)

			reg = regexp.MustCompile(`city(.*?)=(.*?)\"(.*?)\"`)
			city := reg.FindString(string(content))
			city = strings.Replace(city, "\"", "", -1)
			city = strings.Replace(city, "city=", "", -1)

			reg = regexp.MustCompile(`sex(.*?)=(.*?)\"(.*?)\"`)
			sex := reg.FindString(string(content))
			sex = strings.Replace(sex, "\"", "", -1)
			sex = strings.Replace(sex, "sex=", "", -1)
			sexInt, _ := strconv.Atoi(sex)

			receiveMsg.AddFriend.SourceWechat = sourceusername
			receiveMsg.AddFriend.SourceNick = sourcenickname
			receiveMsg.AddFriend.UserWxid = fromusername
			receiveMsg.AddFriend.UserWechat = alias
			if receiveMsg.AddFriend.UserWechat == "" {
				receiveMsg.AddFriend.UserWechat = fromusername
			}
			//if fromusername != "" {
			//	receiveMsg.AddFriend.UserWechat = fromusername
			//} else {
			//	receiveMsg.AddFriend.UserWechat = alias
			//}
			receiveMsg.AddFriend.UserNick = fromnickname

			uf := &UserFriend{
				Alias:    alias,
				City:     city,
				NickName: nickName,
				Sex:      sexInt,
				UserName: userName,
			}
			self.Contact.Friends[userName] = uf
			self.Contact.NickFriends[nickName] = uf
		}
		if receiveMsg.BaseInfo.ReceiveEvent != "" {
			//s, _ := json.Marshal(receiveMsg)
			//logrus.Debug(string(s))
			self.wxh.ReceiveMsg(receiveMsg)
		}
	}
}
