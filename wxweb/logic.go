package wxweb

import (
	//"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

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

	//logrus.Debugf("msg: %v", msgSource)
	modContactList := msgSource["ModContactList"]
	if modContactList != nil {
		//logrus.Debugf("modContactList: %v", modContactList)
		contactList := modContactList.([]interface{})
		if contactList != nil {
			for _, v := range contactList {
				modContact := v.(map[string]interface{})
				if modContact == nil {
					continue
				}
				userName := modContact["UserName"].(string)
				if strings.HasPrefix(userName, GROUP_PREFIX) {
					// 群或者群成员变化
					groupContactFlag := modContact["ContactFlag"].(int)
					groupNickName := modContact["NickName"].(string)
					if !self.argv.IfNotReplaceEmoji {
						groupNickName = replaceEmoji(groupNickName)
					}
					group := self.Contact.GetGroup(userName)
					if group == nil {
						group = NewUserGroup(groupContactFlag, groupNickName, userName, self)
					} else {
						group.ContactFlag = groupContactFlag
						if group.NickName != groupNickName {
							if self.argv.IfNotChangeGroupName {
								// 不准修改群名
								self.WebwxupdatechatroomModTopic(userName, group.NickName)
							} else {
								group.NickName = groupNickName
							}
						}
					}
					memberList := modContact["MemberList"].([]interface{})
					memberListMap := make(map[string]*GroupUserInfo)
					nickMemberListMap := make(map[string]*GroupUserInfo)
					var originalMemberList []*GroupUserInfo
					for _, v2 := range memberList {
						member := v2.(map[string]interface{})
						if member == nil {
							logrus.Errorf("handlemsg get member[%v] error", v2)
							continue
						}
						displayName := member["DisplayName"].(string)
						nickName := member["NickName"].(string)
						if !self.argv.IfNotReplaceEmoji {
							nickName = replaceEmoji(nickName)
						}
						userName := member["UserName"].(string)
						gui := &GroupUserInfo{
							DisplayName: displayName,
							NickName:    nickName,
							UserName:    userName,
						}
						memberListMap[userName] = gui
						nickMemberListMap[nickName] = gui
						if self.argv.IfSaveGroupMember {
							originalMemberList = append(originalMemberList, gui)
						}
					}
					group.ModMember(memberListMap)
					group.SetMemberList(memberListMap, nickMemberListMap, originalMemberList)
					if self.argv.IfSaveGroupMember {
						self.agml.AddGroup(userName)
					}
					self.Contact.SetGroup(userName, group)
					self.Contact.SetNickGroup(groupNickName, group)

					// test
					//if groupNickName == "xxxx" {
					//	logrus.Debugf("mod group - xxxx: %v", group)
					//	for _, v := range group.MemberList {
					//		logrus.Debugf("\tmod group - member: %v", v)
					//	}
					//}
				} else {
					// 新好友
					logrus.Debugf("new friend: %v", modContact)
					userContactFlag := modContact["ContactFlag"].(int)
					userVerifyFlag := modContact["VerifyFlag"].(int)
					userNickName := modContact["NickName"].(string)
					if !self.argv.IfNotReplaceEmoji {
						userNickName = replaceEmoji(userNickName)
					}
					alias := modContact["Alias"].(string)
					city := modContact["City"].(string)
					sex := modContact["Sex"].(int)
					//user := self.Contact.Friends[userName]
					//if user == nil {
						//realName := userNickName
						//_, ok := self.Contact.NickFriends[realName]
						//if ok {
						//	realName = fmt.Sprintf("%s_$$_%d", realName, time.Now().Unix())
						//	self.WebwxOplog(userName, realName)
						//}

					realName := userNickName
					realNickName := fmt.Sprintf("%s__%s", userNickName, time.Now().Format("20060102_15:04"))
					ok := self.WebwxOplog(userName, realNickName)
					if !ok {
						logrus.Errorf("nick[%s] webwxoplog realname[%s] error", userNickName, realNickName)
					} else {
						logrus.Debugf("mod contact webwxoplog success.")
						realName = realNickName
					}

					uf := &UserFriend{
						Alias:       alias,
						City:        city,
						VerifyFlag:  userVerifyFlag,
						ContactFlag: userContactFlag,
						NickName:    userNickName,
						RemarkName:  realName,
						Sex:         sex,
						UserName:    userName,
					}
					self.Contact.Friends[userName] = uf
					self.Contact.NickFriends[realName] = uf

					receiveMsg := &ReceiveMsgInfo{}
					receiveMsg.BaseInfo.Uin = self.Session.Uin
					receiveMsg.BaseInfo.UserName = self.Session.MyUserName
					receiveMsg.BaseInfo.WechatNick = self.Session.MyNickName
					receiveMsg.BaseInfo.FromNickName = realName
					receiveMsg.BaseInfo.FromUserName = userName
					receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_ADD
					receiveMsg.BaseInfo.FromType = FROM_TYPE_PEOPLE
					receiveMsg.AddFriend.UserWechat = uf.Alias
					receiveMsg.AddFriend.UserNick = realName
					receiveMsg.AddFriend.UserCity = uf.City
					receiveMsg.AddFriend.UserSex = uf.Sex
					if receiveMsg.BaseInfo.ReceiveEvent != "" {
						self.wxh.ReceiveMsg(receiveMsg)
					}
					//}
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
		//logrus.Debugf("msg: %v", msg)
		msgType := msg["MsgType"].(int)
		fromUserName := msg["FromUserName"].(string)
		content := msg["Content"].(string)
		content = strings.Replace(content, "&lt;", "<", -1)
		content = strings.Replace(content, "&gt;", ">", -1)
		content = strings.Replace(content, " ", " ", 1)
		if !self.argv.IfNotReplaceEmoji {
			content = replaceEmoji(content)
		}
		msgid := msg["MsgId"].(string)
		receiveMsg := &ReceiveMsgInfo{}
		receiveMsg.BaseInfo.Uin = self.Session.Uin
		receiveMsg.BaseInfo.UserName = self.Session.MyUserName
		receiveMsg.BaseInfo.WechatNick = self.Session.MyNickName
		receiveMsg.BaseInfo.FromUserName = fromUserName
		// 文本消息
		if msgType == MSG_TYPE_TEXT ||
			msgType == MSG_TYPE_IMG ||
			msgType == MSG_TYPE_VOICE ||
			msgType == MSG_TYPE_VIDEO ||
			msgType == MSG_TYPE_CARD ||
			msgType == MSG_TYPE_SHARE_URL {
			// check share url app msg type
			if msgType == MSG_TYPE_SHARE_URL {
				appMsgType := msg["AppMsgType"].(int)
				if appMsgType == MSG_TYPE_TRANSFER {
					msgType = MSG_TYPE_TRANSFER
				}
			}
			receiveMsg.MsgType = RECEIVE_MSG_MAP[msgType]
			if msgType != MSG_TYPE_TRANSFER && strings.Contains(content, MSG_MEDIA_KEYWORD) {
				continue
			}
			if strings.HasPrefix(fromUserName, GROUP_PREFIX) {
				contentSlice := strings.Split(content, ":<br/>")
				if len(contentSlice) < 2 {
					continue
				}
				people := contentSlice[0]
				content = contentSlice[1]
				group := self.Contact.GetGroup(fromUserName)
				if group == nil {
					logrus.Errorf("cannot found the group[%s]", fromUserName)
					continue
				}
				sendPeople := group.GetMemberFromList(people)
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

				peopleNickname := sendPeople.NickName
				uf := self.Contact.GetFriend(people)
				if uf != nil {
					peopleNickname = uf.RemarkName
				}

				receiveMsg.BaseInfo.FromGroupName = group.NickName
				receiveMsg.BaseInfo.FromMemberUserName = sendPeople.UserName
				receiveMsg.BaseInfo.FromNickName = peopleNickname
				receiveMsg.BaseInfo.FromType = FROM_TYPE_GROUP
				receiveMsg.GroupMemberNum = group.GetGroupMemberLen()
			} else {
				if receiveMsg.BaseInfo.FromUserName == self.Session.MyUserName {
					receiveMsg.BaseInfo.FromNickName = self.Session.MyNickName
					toUserName := msg["ToUserName"].(string)
					receiveMsg.BaseToUserInfo.ToUserName = toUserName
					uf, ok := self.Contact.Friends[toUserName]
					if ok {
						receiveMsg.BaseToUserInfo.ToNickName = uf.RemarkName
					}
				} else {
					uf, ok := self.Contact.Friends[receiveMsg.BaseInfo.FromUserName]
					if ok {
						receiveMsg.BaseInfo.FromNickName = uf.RemarkName
					}
				}
				receiveMsg.BaseInfo.FromType = FROM_TYPE_PEOPLE
				self.webwxstatusnotifyMsgRead(receiveMsg.BaseInfo.FromUserName)
			}
			receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_MSG
			switch msgType {
			case MSG_TYPE_TEXT:
				receiveMsg.Msg = content
			case MSG_TYPE_CARD, MSG_TYPE_SHARE_URL, MSG_TYPE_TRANSFER:
				receiveMsg.Msg = RECEIVE_MSG_CONTENT_MAP[msgType]
			case MSG_TYPE_IMG, MSG_TYPE_VIDEO, MSG_TYPE_VOICE:
				receiveMsg.Msg = RECEIVE_MSG_CONTENT_MAP[msgType]
				receiveMsg.MediaTempUrl = self.msgUrlMap[msgType](msgid)
				//self.WebwxsendmsgTransfer(self.TestUserName, content, msgType)
			default:
				receiveMsg.Msg = "unknown msg"
			}
		} else if msgType == MSG_TYPE_INIT {
			//logrus.Debug("[*] 成功截获微信初始化消息", msg)
			statusNotifyCode := msg["StatusNotifyCode"]
			if statusNotifyCode == nil {
				continue
			}
			if statusNotifyCode.(int) != 4 {
				continue
			}
			statusNotifyUserName := msg["StatusNotifyUserName"]
			if statusNotifyUserName == nil {
				continue
			}
			statusNotifyUserNameStr := statusNotifyUserName.(string)
			self.getBigContactList(strings.Split(statusNotifyUserNameStr, ","))
		} else if msgType == MSG_TYPE_SYSTEM {
			logrus.Debugf("系统消息: %s", content)
			// 系统消息,群: 扫描, 邀请
			if strings.Contains(content, WX_SYSTEM_MSG_INVITE) || strings.Contains(content, WX_SYSTEM_MSG_SCAN) {
				group := self.Contact.GetGroup(fromUserName)
				if group == nil {
					continue
				}
				receiveMsg := &ReceiveMsgInfo{}
				receiveMsg.BaseInfo.Uin = self.Session.Uin
				receiveMsg.BaseInfo.UserName = self.Session.MyUserName
				receiveMsg.BaseInfo.WechatNick = self.Session.MyNickName
				receiveMsg.BaseInfo.FromGroupName = group.NickName
				receiveMsg.BaseInfo.FromUserName = fromUserName
				receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_MOD_GROUP_ADD
				receiveMsg.BaseInfo.FromType = FROM_TYPE_GROUP
				receiveMsg.GroupMemberNum = group.GetGroupMemberLen()
				self.wxh.ReceiveMsg(receiveMsg)
			}

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
			
			if strings.Contains(content, WX_SYSTEM_MSG_RED_PACKET) {
				receiveMsg := &ReceiveMsgInfo{}
				receiveMsg.BaseInfo.Uin = self.Session.Uin
				receiveMsg.BaseInfo.UserName = self.Session.MyUserName
				receiveMsg.BaseInfo.WechatNick = self.Session.MyNickName
				receiveMsg.BaseInfo.FromUserName = fromUserName
				receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_MSG
				receiveMsg.BaseInfo.FromType = FROM_TYPE_PEOPLE
				if receiveMsg.BaseInfo.FromUserName == self.Session.MyUserName {
					receiveMsg.BaseInfo.FromNickName = self.Session.MyNickName
					toUserName := msg["ToUserName"].(string)
					receiveMsg.BaseToUserInfo.ToUserName = toUserName
					uf, ok := self.Contact.Friends[toUserName]
					if ok {
						receiveMsg.BaseToUserInfo.ToNickName = uf.RemarkName
					}
				} else {
					uf, ok := self.Contact.Friends[receiveMsg.BaseInfo.FromUserName]
					if ok {
						receiveMsg.BaseInfo.FromNickName = uf.RemarkName
					}
				}
				receiveMsg.MsgType = RECEIVE_MSG_TYPE_RED_PACKET
				receiveMsg.Msg = content
				self.wxh.ReceiveMsg(receiveMsg)
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

			if !self.argv.IfNotReplaceEmoji {
				nickName = replaceEmoji(nickName)
				sourcenickname = replaceEmoji(sourcenickname)
			}

			realName := nickName
			//_, ok := self.Contact.NickFriends[realName]
			//if ok {
			//	realName = fmt.Sprintf("%s_$$_%d", realName, time.Now().Unix())
			//	self.WebwxOplog(userName, realName)
			//}
			//realNickName := fmt.Sprintf("%s_$_%s_$_%s", nickName, self.Session.MyNickName, time.Now().Format("20060102_15:04"))
			//res, ok := self.WebwxOplog(userName, realNickName)
			//if !ok {
			//	logrus.Errorf("nick[%s] webwxoplog realname[%s] error", nickName, realNickName)
			//} else {
			//	if CheckWebwxRetcode(res) {
			//		realName = realNickName
			//	}
			//}

			receiveMsg.BaseInfo.ReceiveEvent = RECEIVE_EVENT_ADD_FRIEND
			receiveMsg.BaseInfo.FromNickName = realName
			receiveMsg.BaseInfo.FromUserName = userName
			receiveMsg.BaseInfo.FromType = FROM_TYPE_PEOPLE
			receiveMsg.AddFriend.Ticket = ticket

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
			receiveMsg.AddFriend.UserNick = realName
			receiveMsg.AddFriend.UserCity = city
			receiveMsg.AddFriend.UserSex = sexInt

			uf := &UserFriend{
				Alias:      alias,
				City:       city,
				NickName:   nickName,
				RemarkName: realName,
				Sex:        sexInt,
				UserName:   userName,
			}
			self.Contact.Friends[userName] = uf
			self.Contact.NickFriends[realName] = uf
		}
		//logrus.Debugf("receiveMsg: %v", receiveMsg)
		if receiveMsg.BaseInfo.ReceiveEvent != "" {
			self.wxh.ReceiveMsg(receiveMsg)
		}
	}
}

func (self *WxWeb) getMsgImgUrl(msgId string) string {
	return fmt.Sprintf("%s/webwxgetmsgimg?MsgID=%s&skey=%s", self.Session.BaseUri, msgId, url.QueryEscape(self.Session.SKey))
}

func (self *WxWeb) getMsgVoiceUrl(msgId string) string {
	return fmt.Sprintf("%s/webwxgetvoice?msgid=%s&skey=%s", self.Session.BaseUri, msgId, url.QueryEscape(self.Session.SKey))
}

func (self *WxWeb) getMsgVideoUrl(msgId string) string {
	return fmt.Sprintf("%s/webwxgetvideo?msgid=%s&skey=%s", self.Session.BaseUri, msgId, url.QueryEscape(self.Session.SKey))
}

func (self *WxWeb) getBigContactList(usernameList []string) {
	logrus.Debugf("get big contact list len: %d", len(usernameList))
	var needGetList []string
	for _, v := range usernameList {
		if strings.HasPrefix(v, GROUP_PREFIX) {
			group := self.Contact.GetGroup(v)
			if group == nil {
				needGetList = append(needGetList, v)
				if len(needGetList) >= 50 {
					logrus.Debugf("big batch get contact len: %d", len(needGetList))
					ok := self.webwxbatchgetcontact(needGetList)
					if !ok {
						logrus.Errorf("webwxbatchgetcontact get error for [%v].", needGetList)
					}
					needGetList = nil
				}
			}
		} else {
			_, ok := self.Contact.Friends[v]
			if !ok {
				needGetList = append(needGetList, v)
				if len(needGetList) >= 50 {
					ok = self.webwxbatchgetcontact(needGetList)
					if !ok {
						logrus.Errorf("webwxbatchgetcontact get error for [%v].", needGetList)
					}
					needGetList = nil
				}
			}
		}
	}
	if needGetList != nil {
		logrus.Debugf("big batch get contact len: %d", len(needGetList))
		ok := self.webwxbatchgetcontact(needGetList)
		if !ok {
			logrus.Errorf("webwxbatchgetcontact get error for [%v].", needGetList)
		}
		needGetList = nil
	}
	self.Contact.PrintGroupInfo()
}

func CheckWebwxResData(res string) (map[string]interface{}, bool) {
	dataRes := JsonDecode(res)
	if dataRes == nil {
		logrus.Errorf("check webwx ret not ok, dataRes == nil, resdata[%s]", res)
		return nil, false
	}
	data := dataRes.(map[string]interface{})
	if data == nil {
		logrus.Errorf("check webwx ret not ok, data == nil, resdata[%s]", res)
		return nil, false
	}
	return data, true
}

func CheckWebwxRetcodeFromData(data map[string]interface{}) bool {
	baseResponse := data["BaseResponse"]
	if baseResponse == nil {
		logrus.Errorf("check webwx ret not ok, baseResponse == nil, data[%v]", data)
		return false
	}
	baseResponseMap := baseResponse.(map[string]interface{})
	if baseResponseMap == nil {
		logrus.Errorf("check webwx ret not ok, baseResponseMap == nil, data[%v]", data)
		return false
	}
	ret := baseResponseMap["Ret"]
	if ret == nil {
		logrus.Errorf("check webwx ret not ok, ret == nil, data[%v]", data)
		return false
	}
	retCode := ret.(int)
	if retCode == WX_RET_SUCCESS {
		return true
	}
	logrus.Errorf("check webwx retcode ret[%d] not ok, data[%v]", retCode, data)
	return false
}

func CheckWebwxRetcode(res string) bool {
	dataRes := JsonDecode(res)
	if dataRes == nil {
		logrus.Errorf("check webwx ret not ok, dataRes == nil, resdata[%s]", res)
		return false
	}
	data := dataRes.(map[string]interface{})
	if data == nil {
		logrus.Errorf("check webwx ret not ok, data == nil, resdata[%s]", res)
		return false
	}
	baseResponse := data["BaseResponse"]
	if baseResponse == nil {
		logrus.Errorf("check webwx ret not ok, baseResponse == nil, resdata[%s]", res)
		return false
	}
	baseResponseMap := baseResponse.(map[string]interface{})
	if baseResponseMap == nil {
		logrus.Errorf("check webwx ret not ok, baseResponseMap == nil, resdata[%s]", res)
		return false
	}
	ret := baseResponseMap["Ret"]
	if ret == nil {
		logrus.Errorf("check webwx ret not ok, ret == nil, resdata[%s]", res)
		return false
	}
	retCode := ret.(int)
	if retCode == WX_RET_SUCCESS {
		return true
	}
	logrus.Errorf("check webwx retcode ret[%d] not ok, resdata[%s]", retCode, res)
	return false
}
