package wxweb

import (
	"fmt"
	"crypto/tls"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"
	"regexp"
	"os"
	"os/exec"
	"io"
	"bytes"
	"runtime"
	"encoding/xml"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/models"
)

type StartWxArgv struct {
	IfInvite           bool   `json:"ifInvite,omitempty"`
	IfInviteEndExit    bool   `json:"inviteEndExit,omitempty"`
	InviteMsg          string `json:"inviteMsg,omitempty"`
	IfClearWx          bool   `json:"ifClearWx,omitempty"`
	ClearWxMsg         string `json:"clearWxMsg,omitempty"`
	ClearWxPrefix      string `json:"clearWxPrefix,omitempty"`
	IfSaveRobotFriends bool   `json:"ifSaveRobotFriends,omitempty"`
	IfSaveRobotGroups  bool   `json:"ifSaveRobotGroups,omitempty"`
	// 是否替换emoji
	IfNotReplaceEmoji bool `json:"ifNotReplaceEmoji,omitempty"`
	// 建群逻辑
	IfCreateGroup     bool     `json:"ifCreateGroup,omitempty"`
	CreateGroupPrefix string   `json:"createGroupPrefix,omitempty"`
	CreateGroupStart  int      `json:"createGroupStart,omitempty"`
	CreateGroupNum    int      `json:"createGroupNum,omitempty"`
	CreateGroupUsers  []string `json:"createGroupUsers,omitempty"`
	// 不准修改群名
	IfNotChangeGroupName bool `json:"ifNotChangeGroupName,omitempty"`
	// 群加人逻辑
	IfSaveGroupMember         bool  `json:"ifSaveGroupMember,omitempty"`
	AddGroupMemberCycleOfTime int64 `json:"addGroupMemberCycleOfTime,omitempty"`
	AddGroupMemberCycleOfNum  int64 `json:"addGroupMemberCycleOfNum,omitempty"`
}

func (self *WxWeb) _run(desc string, f func(...interface{}) bool, args ...interface{}) bool {
	start := time.Now().UnixNano()
	logrus.Infof("%s [%s]", desc, self.Session.MyNickName)
	var result bool
	if len(args) > 1 {
		result = f(args)
	} else if len(args) == 1 {
		result = f(args[0])
	} else {
		result = f()
	}
	useTime := fmt.Sprintf("%.5f", (float64(time.Now().UnixNano()-start) / 1000000000))
	if result {
		logrus.Infof("\t成功,用时 %s 秒", useTime)
	} else {
		logrus.Errorf("%s 失败, 退出该微信", desc)
		self.ifLogin = false
		self.ifLogout = true
		return false
	}
	return true
}

func (self *WxWeb) _init() {
	logrus.SetLevel(logrus.DebugLevel)

	gCookieJar, err := cookiejar.New(nil)
	if err != nil {
		logrus.Fatalf("cookiejar new error: %v", err)
	}

	transport := http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Minute,
		}).Dial,
		TLSHandshakeTimeout: time.Minute,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	httpclient := http.Client{
		Transport: &transport,
		Jar:       gCookieJar,
		Timeout:   time.Minute,
	}

	self.httpClient = &httpclient
	rand.Seed(time.Now().Unix())
	str := strconv.Itoa(rand.Int())
	self.Session.DeviceId = "e" + str[2:17]
	self.Contact = NewUserContact(self)
	self.agml = NewAddGroupMember(self.Contact, self)
}

func (self *WxWeb) getUuid(args ...interface{}) bool {
	urlstr := "https://login.weixin.qq.com/jslogin"
	urlstr += "?appid=wx782c26e4c19acffb&fun=new&lang=zh_CN&_=" + self._unixStr()
	data, _ := self._get(urlstr, false)
	logrus.Debugf("get uuid url[%s] data:%s", urlstr, data)
	re := regexp.MustCompile(`"([\S]+)"`)
	find := re.FindStringSubmatch(data)
	if len(find) > 1 {
		self.Session.Uuid = find[1]
		return true
	} else {
		return false
	}
}

func (self *WxWeb) genQRcode(args ...interface{}) bool {
	urlstr := "https://login.weixin.qq.com/qrcode/" + self.Session.Uuid
	urlstr += "?t=webwx"
	urlstr += "&_=" + self._unixStr()
	self.QrcodeUrl = urlstr
	logrus.Debugf("start wx qrcode url: %s", self.QrcodeUrl)
	self.Session.QrcodePath = self.cfg.QRCodeDir + self.Session.Uuid + ".jpg"
	out, err := os.Create(self.Session.QrcodePath)
	resp, err := self._get(urlstr, false)
	_, err = io.Copy(out, bytes.NewReader([]byte(resp)))
	if err != nil {
		return false
	} else {
		if runtime.GOOS == "darwin" {
			exec.Command("open", self.Session.QrcodePath).Run()
		}
		return true
	}
}

func (self *WxWeb) waitForLogin(tip int) bool {
	// https://login.wx.qq.com/cgi-bin/mmwebwx-bin/login?loginicon=true&uuid=wed6_NBBLA==&tip=0&r=1997008698&_=1488356620032
	time.Sleep(time.Duration(tip) * time.Second)
	//url := "https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login"
	url := "https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login"
	url += "?tip=" + strconv.Itoa(tip) + "&uuid=" + self.Session.Uuid + "&_=" + self._unixStr()
	logrus.Debugf("wait for login url: %s", url)
	data, err := self._get(url, false)
	if err != nil {
		logrus.Errorf("wait for login error: %v", err)
		return false
	}
	logrus.Debug(data)
	re := regexp.MustCompile(`window.code=(\d+);`)
	find := re.FindStringSubmatch(data)
	if len(find) > 1 {
		code := find[1]
		if code == "201" {
			if tip == 0 {
				return false
			}
			return true
		} else if code == "200" {
			re := regexp.MustCompile(`window.redirect_uri="(\S+?)";`)
			find := re.FindStringSubmatch(data)
			if len(find) > 1 {
				r_uri := find[1] + "&fun=new"
				self.Session.RedirectUri = r_uri
				re = regexp.MustCompile(`/`)
				finded := re.FindAllStringIndex(r_uri, -1)
				self.Session.BaseUri = r_uri[:finded[len(finded)-1][0]]
				self.Session.BaseHost = self.Session.BaseUri[8:]
				self.Session.BaseHost = self.Session.BaseHost[:strings.Index(self.Session.BaseHost, "/")]
				logrus.Debugf("webwx base uri: %s", self.Session.BaseHost)
				return true
			}
			return false
		} else if code == "408" {
			logrus.Errorf("uuid[%s] [登陆超时]", self.Session.Uuid)
		} else {
			logrus.Errorf("uuid[%s] [登陆异常]", self.Session.Uuid)
		}
	}
	return false
}

func (self *WxWeb) login(args ...interface{}) bool {
	logrus.Debugf("login redirect uri: %s", self.Session.RedirectUri)
	if self.Session.RedirectUri == "" {
		return false
	}
	data, err := self._get(self.Session.RedirectUri, false)
	if err != nil {
		logrus.Errorf("wx login from [%s] error: %v", self.Session.RedirectUri, err)
		return false
	}
	type Result struct {
		Ret        int    `xml:"ret"`
		Message    string `xml:"message"`
		Skey       string `xml:"skey"`
		Wxsid      string `xml:"wxsid"`
		Wxuin      string `xml:"wxuin"`
		PassTicket string `xml:"pass_ticket"`
	}
	v := Result{}
	err = xml.Unmarshal([]byte(data), &v)
	if err != nil {
		logrus.Errorf("wx login unmarshal error: %v", err)
		return false
	}
	if v.Ret != 0 {
		logrus.Errorf("wx login failed error message: %v", v)
		return false
	}
	self.Session.SKey = v.Skey
	self.Session.Sid = v.Wxsid
	self.Session.Uin = v.Wxuin
	self.Session.PassTicket = v.PassTicket
	self.Session.BaseRequest = make(map[string]interface{})
	self.Session.BaseRequest["Uin"], _ = strconv.Atoi(v.Wxuin)
	self.Session.BaseRequest["Sid"] = v.Wxsid
	self.Session.BaseRequest["Skey"] = v.Skey
	self.Session.BaseRequest["DeviceID"] = self.Session.DeviceId
	
	return true
}

func (self *WxWeb) Resume(robot *models.Robot) bool {
	if self.argv == nil {
		self.argv = &StartWxArgv{}
	}
	
	self.startTime = time.Now().Unix()
	
	self.Lock()
	self.enable = true
	self.Unlock()
	
	// 初始化
	self._init()
	
	ok := self.quickLogin(robot)
	if !ok {
		logrus.Debugf("resume robot[%s] quick login failed", robot.RobotWx)
		return false
	}
	go self.wechatLoop()
	
	return true
}

func (self *WxWeb) Start() {
	if self.argv == nil {
		self.argv = &StartWxArgv{}
	}

	self.startTime = time.Now().Unix()

	self.Lock()
	self.enable = true
	self.Unlock()

	// 初始化
	self._init()
	
	logrus.Info("[*] 开启微信网页版 ...")
	self._run("[*] 正在获取 uuid ... ", self.getUuid)
	self._run("[*] 正在获取 二维码 ... ", self.genQRcode)
	logrus.Infof("[*] 请使用微信扫描二维码以登录 ... uuid[%s]", self.Session.Uuid)

	go self.Run()
}

func (self *WxWeb) quickLogin(robot *models.Robot) bool {
	ok := self.parseSessionCache(robot)
	if !ok {
		logrus.Debugf("[quick login] error, maybe need login with scan.")
		return false
	}
	logrus.Infof("[*] 正在快速登录微信, 初始化 ... ")
	ok = self.wechatInit()
	if !ok {
		return false
	}
	logrus.Infof("[quick login] success [%s]", self.Session.MyNickName)

	return true
}

func (self *WxWeb) Run() {
	for {
		if !self.enable {
			close(self.stopped)
			return
		}
		if self.waitForLogin(1) == false {
			continue
		}
		logrus.Infof("[*] 请在手机上点击确认以登录 ... ")
		if self.waitForLogin(0) == false {
			continue
		}
		break
	}
	ok := self._run("[*] 正在登录 ... ", self.login)
	if !ok {
		return
	}
	ok = self.wechatInit()
	if !ok {
		return
	}
	self.wxh.Login(self.Session.Uuid)
	
	self.wechatLoop()
}

func (self *WxWeb) wechatInit() bool {
	ok := self._run("[*] 微信初始化 ... ", self.webwxinit)
	if !ok {
		return false
	}
	ok = self._run("[*] 开启状态通知 ... ", self.webwxstatusnotify)
	if !ok {
		return false
	}
	ok = self._run("[*] 进行同步线路测试 ... ", self.testsynccheck)
	if !ok {
		return false
	}
	ok = self._run("[*] 获取好友列表 ... ", self.webwxgetcontact)
	if !ok {
		return false
	}
	ok = self._run("[*] 获取群列表 ... ", self.GroupWebwxbatchgetcontact)
	if !ok {
		return false
	}
	return true
}

func (self *WxWeb) wechatLoop() {
	if self.argv.IfInvite {
		go self.Contact.InviteMembers()
	}
	if self.argv.IfClearWx {
		go self.Contact.ClearWx()
	}
	if self.argv.IfSaveRobotFriends {
		go self.Contact.SaveRobotFriends()
	}
	if self.argv.IfSaveRobotGroups {
		go self.Contact.SaveRobotGroups()
	}
	if self.argv.IfCreateGroup {
		go self.Contact.CreateGroups()
	}
	
	//self.testUploadMedia()
	//self.Contact.PrintGroupInfo()
	//self.Contact.GroupMass()
	
	self.Lock()
	self.ifLogin = true
	self.Unlock()
	for {
		if !self.enable {
			self.Lock()
			self.ifLogout = true
			self.Unlock()
			self.wxh.Logout(self.Session.Uuid)
			close(self.stopped)
			return
		}
		
		retcode, selector := self.synccheck()
		//logrus.Debugf("sync check recode: %s selector: %s", retcode, selector)
		if retcode == "1100" {
			logrus.Infof("[*] user[%v] 你在手机上登出了微信, 88", self.Session.User)
			self.Lock()
			self.ifLogout = true
			self.Unlock()
			self.wxh.Logout(self.Session.Uuid)
			break
		} else if retcode == "1101" {
			logrus.Infof("[*] user[%v] 你在其他地方登录了 WEB 版微信, 88", self.Session.User)
			self.Lock()
			self.ifLogout = true
			self.Unlock()
			self.wxh.Logout(self.Session.Uuid)
			break
		} else if retcode == "0" {
			// selector: 2 普通消息 6 用户同意好友申请 4 通讯录变更
			if selector == "2" || selector == "6" {
				r := self.webwxsync()
				if r == nil {
					time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
					continue
				}
				//logrus.Debugf("webwxsync: %v", r)
				switch r.(type) {
				case bool:
				default:
					self.handleMsg(r)
				}
				time.Sleep(WEBWX_HANDLE_MSG_SYNC_INTERVAL * time.Second)
			} else if selector == "0" {
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			} else if selector == "4" || selector == "7" || selector == "3" {
				self.webwxsync()
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			} else {
				self.webwxsync()
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			}
		}
	}
}
