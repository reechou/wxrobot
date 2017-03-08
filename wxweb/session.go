package wxweb

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/models"
)

func (self *WxWeb) getSessionCache() (string, []*http.Cookie) {
	robot := &models.Robot{
		RobotWx: self.Session.MyNickName,
	}
	has, err := models.GetRobot(robot)
	if err != nil {
		logrus.Errorf("get robot error: %v", err)
		return "", nil
	}
	if has {
		var cookieInterfaces []interface{}
		err = json.Unmarshal([]byte(robot.WebwxCookie), &cookieInterfaces)
		if err != nil {
			return "", nil
		}

		var cookies []*http.Cookie

		for _, c := range cookieInterfaces {
			b, _ := json.Marshal(c)
			var cookie *http.Cookie
			e := json.Unmarshal(b, cookie)
			if e == nil {
				cookies = append(cookies, cookie)
			}
		}

		return robot.BaseLoginInfo, cookies
	}

	return "", nil
}

func (self *WxWeb) checkSession(cookies []*http.Cookie) {
	now := time.Now().Unix()
	if now-self.lastSaveCookieTime > 300 {
		self.refreshSessionCache(cookies)
		self.lastSaveCookieTime = now
	}
}

func (self *WxWeb) refreshSessionCache(cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}

	b, err := json.Marshal(cookies)
	if err != nil {
		logrus.Errorf("refresh cookie error: %v", err)
	} else {
		robot := &models.Robot{
			RobotWx: self.Session.MyNickName,
		}
		has, err := models.GetRobot(robot)
		if err != nil {
			logrus.Errorf("get robot error: %v", err)
			return
		}
		if has {
			robot.BaseLoginInfo = self.Session.RedirectUri
			robot.WebwxCookie = string(b)
			err = models.UpdateRobotSession(robot)
			if err != nil {
				logrus.Errorf("update robot session error: %v", err)
			}
		}
	}
}
