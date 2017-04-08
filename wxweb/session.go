package wxweb

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/models"
)

func (self *WxWeb) parseSessionCache(robot *models.Robot) bool {
	if robot.BaseLoginInfo == "" || robot.WebwxCookie == "" {
		logrus.Debugf("cannot found robot session baselogininfo or cookie.")
		return false
	}

	var cookieInterfaces []interface{}
	err := json.Unmarshal([]byte(robot.WebwxCookie), &cookieInterfaces)
	if err != nil {
		logrus.Errorf("json unmarshal cookie error: %v", err)
		return false
	}

	err = json.Unmarshal([]byte(robot.BaseLoginInfo), &self.Session)
	if err != nil {
		logrus.Errorf("json unmarshal base login info error: %v", err)
		return false
	}
	logrus.Debugf("parse baselogin info session success: %v", self.Session)

	if robot.Argv != "" {
		err = json.Unmarshal([]byte(robot.Argv), &self.argv)
		if err != nil {
			logrus.Errorf("json unmarshal argv error: %v", err)
			return false
		}
		logrus.Debugf("parse robot argv session success: %v", self.argv)
	}

	var cookies []*http.Cookie
	for _, c := range cookieInterfaces {
		b, _ := json.Marshal(c)
		var cookie http.Cookie
		e := json.Unmarshal(b, &cookie)
		if e == nil {
			cookies = append(cookies, &cookie)
		}
	}
	self.cookies = cookies
	u, err := url.Parse(self.Session.BaseUri)
	if err != nil {
		logrus.Errorf("url parse error: %v", err)
		return false
	}
	self.httpClient.Jar.SetCookies(u, cookies)
	if uin, ok := self.Session.BaseRequest["Uin"].(float64); ok {
		self.Session.BaseRequest["Uin"] = int64(uin)
	}

	return true
}

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
	if len(cookies) == 0 || self.Session.MyNickName == "" {
		return
	}

	//self.refreshSessionCache(cookies)
	now := time.Now().Unix()
	if now-self.lastSaveCookieTime > 60 {
		self.refreshSessionCache(cookies)
		self.lastSaveCookieTime = now
	}
}

func (self *WxWeb) refreshSessionCache(cookies []*http.Cookie) {
	cookiesCache, err := json.Marshal(cookies)
	if err != nil {
		logrus.Errorf("refresh json marshal cookie error: %v", err)
		return
	}
	baseInfoCache, err := json.Marshal(self.Session)
	if err != nil {
		logrus.Errorf("refresh json marshal base info error: %v", err)
		return
	}
	robot := &models.Robot{
		RobotWx: self.Session.MyNickName,
	}
	has, err := models.GetRobot(robot)
	if err != nil {
		logrus.Errorf("get robot error: %v", err)
		return
	}
	if has {
		robot.BaseLoginInfo = string(baseInfoCache)
		robot.WebwxCookie = string(cookiesCache)
		err = models.UpdateRobotSession(robot)
		if err != nil {
			logrus.Errorf("update robot session error: %v", err)
			return
		}
	} else {
		robot.Ip = HostIP
		robot.OfPort = self.cfg.Host
		robot.BaseLoginInfo = string(baseInfoCache)
		robot.WebwxCookie = string(cookiesCache)
		err = models.CreateRobot(robot)
		if err != nil {
			logrus.Errorf("create robot error: %v", err)
			return
		}
	}
	logrus.Debugf("[%s] refresh session and cookie cache success.", self.Session.MyNickName)
}

func (self *WxWeb) refreshRobotArgv() {
	argvCache, err := json.Marshal(self.argv)
	if err != nil {
		logrus.Errorf("refresh json marshal robot argv error: %v", err)
		return
	}
	robot := &models.Robot{
		RobotWx: self.Session.MyNickName,
	}
	has, err := models.GetRobot(robot)
	if err != nil {
		logrus.Errorf("get robot error: %v", err)
		return
	}
	robot.RobotType = self.argv.RobotType
	robot.Ip = HostIP
	robot.OfPort = self.cfg.Host
	robot.Argv = string(argvCache)
	if has {
		err = models.UpdateRobotArgv(robot)
		if err != nil {
			logrus.Errorf("update robot argv session error: %v", err)
			return
		}
	} else {
		err = models.CreateRobot(robot)
		if err != nil {
			logrus.Errorf("create robot error: %v", err)
			return
		}
	}
	logrus.Debugf("[%s] refresh robot argv cache success.", self.Session.MyNickName)
}
