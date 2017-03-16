package wxweb

type GroupMember struct {
	GroupUserName string
	Member        *GroupUserInfo
}

type GroupAddFriendLogic struct {
	wx *WxWeb
}

func NewGroupAddFriendLogic(wx *WxWeb) *GroupAddFriendLogic {
	ngaf := &GroupAddFriendLogic{wx: wx}

	return ngaf
}
