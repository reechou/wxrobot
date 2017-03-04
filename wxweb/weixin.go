//web weixin client
package wxweb

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	z "github.com/nutzam/zgo"
	"github.com/reechou/wxrobot/config"
)

const debug = false

func debugPrint(content interface{}) {
	if debug == true {
		fmt.Println(content)
	}
}

type StartWxArgv struct {
	IfInvite             bool     `json:"ifInvite,omitempty"`
	IfInviteEndExit      bool     `json:"inviteEndExit,omitempty"`
	InviteMsg            string   `json:"inviteMsg,omitempty"`
	IfClearWx            bool     `json:"ifClearWx,omitempty"`
	ClearWxMsg           string   `json:"clearWxMsg,omitempty"`
	ClearWxPrefix        string   `json:"clearWxPrefix,omitempty"`
	IfSaveRobot          bool     `json:"ifSaveRobot,omitempty"`
	IfCreateGroup        bool     `json:"ifCreateGroup,omitempty"`
	CreateGroupPrefix    string   `json:"createGroupPrefix,omitempty"`
	CreateGroupStart     int      `json:"createGroupStart,omitempty"`
	CreateGroupNum       int      `json:"createGroupNum,omitempty"`
	CreateGroupUsers     []string `json:"createGroupUsers,omitempty"`
	IfNotChangeGroupName bool     `json:"ifNotChangeGroupName,omitempty"`
}

type WxHandler interface {
	Login(uuid string)
	Logout(uuid string)
	ReceiveMsg(msg *ReceiveMsgInfo)
	RobotAddFriends(robot string, friends []UserFriend)
}

type WebWxSession struct {
	Uuid        string
	BaseUri     string
	BaseHost    string
	RedirectUri string
	Uin         string
	Sid         string
	SKey        string
	PassTicket  string
	DeviceId    string
	SyncKey     map[string]interface{}
	SyncKeyStr  string
	User        map[string]interface{}
	MyNickName  string
	MyUserName  string
	BaseRequest map[string]interface{}
	SyncHost    string
	MediaCount  int64
}

type WxWebMediaInfo struct {
	MediaId  string
	LastTime int64
}

type WxWeb struct {
	sync.Mutex

	Session *WebWxSession

	httpClient     *http.Client
	cookies        []*http.Cookie
	ifTestSyncOK   bool
	ifChangeCookie bool
	SpecialUsers   map[string]int
	TestUserName   string
	QrcodeUrl      string
	QrcodePath     string

	cfg *config.Config

	msgUrlMap map[int]msgUrlHandle
	Contact   *UserContact
	wxh       WxHandler
	argv      *StartWxArgv

	lastSaveCookieTime int64
	
	imgMediaMutex sync.Mutex
	imgMediaIdMap map[string]*WxWebMediaInfo

	startTime int64
	ifLogin   bool
	ifLogout  bool
	enable    bool
	ifCleared bool
	stopped   chan struct{}
}

func NewWxWeb(cfg *config.Config, wxh WxHandler) *WxWeb {
	wx := &WxWeb{
		cfg:     cfg,
		stopped: make(chan struct{}),
		wxh:     wxh,
		Session: &WebWxSession{MediaCount: -1},
		imgMediaIdMap: make(map[string]*WxWebMediaInfo),
	}
	wx.initSpecialUsers()

	return wx
}

func NewWxWebWithArgv(cfg *config.Config, wxh WxHandler, argv *StartWxArgv) *WxWeb {
	wx := &WxWeb{
		cfg:     cfg,
		stopped: make(chan struct{}),
		wxh:     wxh,
		argv:    argv,
		Session: &WebWxSession{MediaCount: -1},
		imgMediaIdMap: make(map[string]*WxWebMediaInfo),
	}
	wx.initMsgUrlMap()
	wx.initSpecialUsers()

	return wx
}

func (self *WxWeb) initMsgUrlMap() {
	self.msgUrlMap = map[int]msgUrlHandle{
		MSG_TYPE_IMG:   self.getMsgImgUrl,
		MSG_TYPE_VOICE: self.getMsgVoiceUrl,
		MSG_TYPE_VIDEO: self.getMsgVideoUrl,
	}
}

func (self *WxWeb) initSpecialUsers() {
	self.SpecialUsers = map[string]int{
		"newsapp":               1,
		"fmessage":              1,
		"filehelper":            1,
		"weibo":                 1,
		"qqmail":                1,
		"tmessage":              1,
		"qmessage":              1,
		"qqsync":                1,
		"floatbottle":           1,
		"lbsapp":                1,
		"shakeapp":              1,
		"medianote":             1,
		"qqfriend":              1,
		"readerapp":             1,
		"blogapp":               1,
		"facebookapp":           1,
		"masssendapp":           1,
		"meishiapp":             1,
		"feedsapp":              1,
		"voip":                  1,
		"blogappweixin":         1,
		"weixin":                1,
		"brandsessionholder":    1,
		"weixinreminder":        1,
		"wxid_novlwrv3lqwv11":   1,
		"gh_22b87fa7cb3c":       1,
		"officialaccounts":      1,
		"notification_messages": 1,
		"wxitil":                1,
		"userexperience_alarm":  1,
		"mphelper":              1,
	}
}

func (self *WxWeb) Stop() {
	self.Lock()
	self.enable = false
	self.Unlock()
	self.Clear()

	<-self.stopped
}

func (self *WxWeb) Clear() {
	self.Lock()
	defer self.Unlock()
	if !self.ifCleared {
		err := os.Remove(self.QrcodePath)
		if err != nil {
			logrus.Errorf("remove qrcode[%s] error: %v", self.QrcodePath, err)
		}
		self.ifCleared = true
	}
}

func (self *WxWeb) UUID() string {
	return self.Session.Uuid
}

func (self *WxWeb) GetUin() string {
	return self.Session.Uin
}

func (self *WxWeb) QRCODE() string {
	return self.QrcodeUrl
}

func (self *WxWeb) QRCODEPath() string {
	return self.QrcodePath
}

func (self *WxWeb) IfLogin() bool {
	return self.ifLogin
}

func (self *WxWeb) IfLogout() bool {
	return self.ifLogout
}

func (self *WxWeb) StartTime() int64 {
	return self.startTime
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

func (self *WxWeb) _run(desc string, f func(...interface{}) bool, args ...interface{}) {
	start := time.Now().UnixNano()
	logrus.Info(desc)
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
		logrus.Errorf("%s\t失败\n[*] 退出该微信", desc)
		self.ifLogin = false
		self.ifLogout = true
	}
}

func (self *WxWeb) _postFile(urlstr string, req *bytes.Buffer) (string, error) {
	var err error
	var resp *http.Response
	request, err := http.NewRequest("POST", urlstr, req)
	if err != nil {
		return "", err
	}
	request.Header.Add("Accept", "*/*")
	request.Header.Add("Content-Type", "multipart/form-data")
	request.Header.Add("Accept-Encoding", "gzip, deflate, br")
	request.Header.Add("Accept-Language", "zh-CN,zh;q=0.8,de;q=0.6,en;q=0.4,ko;q=0.2,pt;q=0.2,zh-TW;q=0.2")
	request.Header.Add("Connection", "keep-alive")
	request.Header.Add("Host", "file."+self.Session.BaseHost)
	request.Header.Add("Origin", "https://"+self.Session.BaseHost)
	request.Header.Add("Referer", "https://"+self.Session.BaseHost+"/?&lang=zh_CN")
	request.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.71 Safari/537.36")
	if self.cookies != nil {
		for _, v := range self.cookies {
			request.AddCookie(v)
		}
	}
	resp, err = self.httpClient.Do(request)

	if err != nil || resp == nil {
		logrus.Error("post file error:", err)
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Error("post file error:", err)
		return "", err
	} else {
		defer resp.Body.Close()
	}
	return string(body), nil
}

func (self *WxWeb) _post(urlstr string, params map[string]interface{}, jsonFmt bool) (string, error) {
	var err error
	var resp *http.Response
	if jsonFmt == true {
		jsonPost := JsonEncode(params)
		debugPrint(jsonPost)
		requestBody := bytes.NewBuffer([]byte(jsonPost))
		request, err := http.NewRequest("POST", urlstr, requestBody)
		if err != nil {
			return "", err
		}
		//fmt.Println(request.URL.Host)
		//for _, v := range self.httpClient.Jar.Cookies(request.URL) {
		//	fmt.Println(v.String())
		//}
		request.Header.Set("Content-Type", "application/json;charset=utf-8")
		request.Header.Add("Referer", "https://"+self.Session.BaseHost)
		request.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.111 Safari/537.36")
		if self.cookies != nil {
			for _, v := range self.cookies {
				request.AddCookie(v)
				//fmt.Println("save : ", v.String())
			}
		}
		resp, err = self.httpClient.Do(request)
	} else {
		v := url.Values{}
		for key, value := range params {
			v.Add(key, value.(string))
		}
		resp, err = self.httpClient.PostForm(urlstr, v)
	}

	if err != nil || resp == nil {
		logrus.Error("post error:", err)
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Error("post error:", err)
		return "", err
	} else {
		defer resp.Body.Close()
	}
	return string(body), nil
}

func (self *WxWeb) _get(urlstr string, jsonFmt bool) (string, error) {
	var err error
	res := ""
	request, err := http.NewRequest("GET", urlstr, nil)
	if err != nil {
		return "", err
	}

	//fmt.Println(request.URL.Host)
	//for _, v := range self.httpClient.Jar.Cookies(request.URL) {
	//	fmt.Println(v.String())
	//}

	request.Header.Add("Referer", "https://wx.qq.com")
	request.Header.Add("User-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.111 Safari/537.36")
	if self.cookies != nil {
		for _, v := range self.cookies {
			request.AddCookie(v)
			//fmt.Println("save : ", v.String())
		}
	}
	resp, err := self.httpClient.Do(request)
	if err != nil {
		return res, err
	}
	if resp.Cookies() != nil && len(resp.Cookies()) > 0 {
		self.cookies = resp.Cookies()
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, err
	}
	return string(body), nil
}

func (self *WxWeb) _unixStr() string {
	return strconv.Itoa(int(time.Now().UnixNano() / 1000000))
}

func (self *WxWeb) genQRcode(args ...interface{}) bool {
	urlstr := "https://login.weixin.qq.com/qrcode/" + self.Session.Uuid
	urlstr += "?t=webwx"
	urlstr += "&_=" + self._unixStr()
	self.QrcodeUrl = urlstr
	logrus.Debugf("start wx qrcode url: %s", self.QrcodeUrl)
	self.QrcodePath = self.cfg.QRCodeDir + self.Session.Uuid + ".jpg"
	out, err := os.Create(self.QrcodePath)
	resp, err := self._get(urlstr, false)
	_, err = io.Copy(out, bytes.NewReader([]byte(resp)))
	if err != nil {
		return false
	} else {
		if runtime.GOOS == "darwin" {
			exec.Command("open", self.QrcodePath).Run()
		}
		//else {
		//	go func() {
		//		fmt.Println("please open on web browser ip:8889/qrcode")
		//		http.HandleFunc("/qrcode", func(w http.ResponseWriter, req *http.Request) {
		//			http.ServeFile(w, req, "qrcode.jpg")
		//			return
		//		})
		//		http.ListenAndServe(":8889", nil)
		//	}()
		//}
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
	data, _ := self._get(self.Session.RedirectUri, false)
	type Result struct {
		Skey       string `xml:"skey"`
		Wxsid      string `xml:"wxsid"`
		Wxuin      string `xml:"wxuin"`
		PassTicket string `xml:"pass_ticket"`
	}
	v := Result{}
	err := xml.Unmarshal([]byte(data), &v)
	if err != nil {
		fmt.Printf("error: %v", err)
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

func (self *WxWeb) webwxinit(args ...interface{}) bool {
	url := fmt.Sprintf("%s/webwxinit?passTicket=%s&skey=%s&r=%s",
		self.Session.BaseUri, self.Session.PassTicket, self.Session.SKey, self._unixStr())
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	res, err := self._post(url, params, true)
	if err != nil {
		return false
	}

	dataJson := JsonDecode(res)
	if dataJson == nil {
		return false
	}
	data := dataJson.(map[string]interface{})
	self.Session.User = data["User"].(map[string]interface{})
	nickName, ok := self.Session.User["NickName"]
	if ok {
		nick := nickName.(string)
		self.Session.MyNickName = nick
	}
	userName, ok := self.Session.User["UserName"]
	if ok {
		myUserName := userName.(string)
		self.Session.MyUserName = myUserName
	}
	self.Session.SyncKey = data["SyncKey"].(map[string]interface{})
	self._setsynckey()

	retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
	if retCode != WX_RET_SUCCESS {
		return false
	}
	chatSet := data["ChatSet"].(string)
	chats := strings.Split(chatSet, ",")
	for _, v := range chats {
		if strings.HasPrefix(v, GROUP_PREFIX) {
			ug := NewUserGroup(0, "", v, self)
			self.Contact.Groups[v] = ug
		}
	}
	logrus.Debugf("webwxinit get group num: %d", len(self.Contact.Groups))

	return true
}

func (self *WxWeb) _setsynckey() {
	keys := []string{}
	for _, keyVal := range self.Session.SyncKey["List"].([]interface{}) {
		key := strconv.Itoa(int(keyVal.(map[string]interface{})["Key"].(int)))
		value := strconv.Itoa(int(keyVal.(map[string]interface{})["Val"].(int)))
		keys = append(keys, key+"_"+value)
	}
	self.Session.SyncKeyStr = strings.Join(keys, "|")
}

func (self *WxWeb) synccheck() (string, string) {
	if self.ifTestSyncOK {
		if !self.ifChangeCookie {
			for _, v := range self.cookies {
				if v.Name == "wxloadtime" {
					v.Value = v.Value + "_expired"
					break
				}
			}
			self.ifChangeCookie = true
		}
	}
	urlstr := fmt.Sprintf("https://%s/cgi-bin/mmwebwx-bin/synccheck", self.Session.SyncHost)
	v := url.Values{}
	v.Add("r", self._unixStr())
	v.Add("sid", self.Session.Sid)
	v.Add("uin", self.Session.Uin)
	v.Add("skey", self.Session.SKey)
	v.Add("deviceid", self.Session.DeviceId)
	v.Add("synckey", self.Session.SyncKeyStr)
	v.Add("_", self._unixStr())
	urlstr = urlstr + "?" + v.Encode()
	data, _ := self._get(urlstr, false)
	if data == "" {
		return "9999", "0"
	}
	re := regexp.MustCompile(`window.synccheck={retcode:"(\d+)",selector:"(\d+)"}`)
	find := re.FindStringSubmatch(data)
	if len(find) > 2 {
		retcode := find[1]
		selector := find[2]
		debugPrint(fmt.Sprintf("retcode:%s,selector,selector%s", find[1], find[2]))
		return retcode, selector
	} else {
		return "9999", "0"
	}
}

func (self *WxWeb) testsynccheck(args ...interface{}) bool {
	SyncHost := []string{
		"webpush.weixin.qq.com",
		"webpush2.weixin.qq.com",
		"webpush.wechat.com",
		"webpush1.wechat.com",
		"webpush2.wechat.com",
		"webpush1.wechatapp.com",
		//"webpush.wechatapp.com"
	}
	for _, host := range SyncHost {
		self.Session.SyncHost = host
		retcode, _ := self.synccheck()
		if retcode == "0" {
			self.ifTestSyncOK = true
			return true
		}
	}
	return false
}

func (self *WxWeb) webwxstatusnotify(args ...interface{}) bool {
	urlstr := fmt.Sprintf("%s/webwxstatusnotify?lang=zh_CN&passTicket=%s",
		self.Session.BaseUri, self.Session.PassTicket)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["Code"] = 3
	params["FromUserName"] = self.Session.User["UserName"]
	params["ToUserName"] = self.Session.User["UserName"]
	params["ClientMsgId"] = int(time.Now().Unix())
	res, err := self._post(urlstr, params, true)
	if err != nil {
		return false
	}
	return CheckWebwxRetcode(res)
	//dataJson := JsonDecode(res)
	//if dataJson == nil {
	//	return false
	//}
	//data := dataJson.(map[string]interface{})
	//retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
	//return retCode == 0
}

func (self *WxWeb) webwxstatusnotifyMsgRead(toUserName string) bool {
	//now := time.Now().Unix()
	//if now-self.msgReadTimestamp < 17 {
	//	return true
	//}
	//self.msgReadTimestamp = now

	urlstr := fmt.Sprintf("%s/webwxstatusnotify", self.Session.BaseUri)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["Code"] = 1
	params["FromUserName"] = self.Session.User["UserName"]
	params["ToUserName"] = toUserName
	params["ClientMsgId"] = self._unixStr()
	res, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("webwxstatusnotifyMsgRead post error: %v", err)
		return false
	}
	return CheckWebwxRetcode(res)
	//dataJson := JsonDecode(res)
	//if dataJson == nil {
	//	logrus.Errorf("webwxstatusnotifyMsgRead JsonDecode datajson == nil")
	//	return false
	//}
	//data := dataJson.(map[string]interface{})
	//logrus.Debugf("[%s] webwxstatusnotifyMsgRead[%s] data: %v", self.Session.MyNickName, toUserName, data)
	//retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
	//return retCode == 0
}

func (self *WxWeb) webwxgetcontact(args ...interface{}) bool {
	maxTime := 0
	seq := 0
	if maxTime == 0 || seq != 0 {
		maxTime++
		if maxTime >= 10 {
			return true
		}
		urlstr := fmt.Sprintf("%s/webwxgetcontact?lang=zh_CN&pass_ticket=%s&seq=%d&skey=%s&r=%s",
			self.Session.BaseUri, self.Session.PassTicket, seq, self.Session.SKey, self._unixStr())
		logrus.Debugf("get contact url[%s] seq: %d maxTime:%d", urlstr, seq, maxTime)
		res, err := self._post(urlstr, nil, true)
		if err != nil {
			logrus.Errorf("webwxgetcontact _post error: %v", err)
			return false
		}

		dataJson := JsonDecode(res)
		if dataJson == nil {
			logrus.Errorf("webwxgetcontact dataJson == nil")
			return false
		}
		data := dataJson.(map[string]interface{})
		if data == nil {
			logrus.Errorf("webwxgetcontact JsonDecode error: %v", err)
			return false
		}
		//logrus.Debugf("get contact info: %v", data)
		retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
		if retCode != WX_RET_SUCCESS {
			logrus.Errorf("webwxgetcontact get error retcode[%d]", retCode)
			return false
		}
		seqV := data["Seq"]
		if seqV != nil {
			seq = seqV.(int)
			logrus.Debugf("get contact get new seq: %d", seq)
		}

		memberList := data["MemberList"].([]interface{})
		if memberList == nil {
			logrus.Errorf("webwxgetcontact get memberList error")
			return false
		}
		for _, v := range memberList {
			member := v.(map[string]interface{})
			if member == nil {
				logrus.Errorf("webwxgetcontact get member[%v] error.", v)
				continue
			}
			userName := member["UserName"].(string)
			contactFlag := member["ContactFlag"].(int)
			nickName := member["NickName"].(string)
			// change emoji
			nickName = replaceEmoji(nickName)
			
			//logrus.Debugf("nickname[%s] username[%s] %v", nickName, userName, member)
			if strings.HasPrefix(userName, GROUP_PREFIX) {
				ug := NewUserGroup(contactFlag, nickName, userName, self)
				self.Contact.Groups[userName] = ug
			} else {
				remarkName := member["RemarkName"].(string)
				alias := member["Alias"].(string)
				city := member["City"].(string)
				sex := member["Sex"].(int)
				verifyFlag := member["VerifyFlag"].(int)
				if verifyFlag == WX_FRIEND_VERIFY_FLAG_DINGYUEHAO || verifyFlag == WX_FRIEND_VERIFY_FLAG_FUWUHAO {
					continue
				}

				realName := remarkName
				if realName == "" {
					realName = nickName
				}
				// change emoji
				realName = replaceEmoji(realName)
				
				_, ok := self.Contact.NickFriends[realName]
				if ok {
					realName = fmt.Sprintf("%s_$_%d", realName, time.Now().Unix())
					self.WebwxOplog(userName, realName)
					time.Sleep(time.Second)
				}

				uf := &UserFriend{
					Alias:       alias,
					City:        city,
					VerifyFlag:  verifyFlag,
					ContactFlag: contactFlag,
					NickName:    nickName,
					RemarkName:  realName,
					Sex:         sex,
					UserName:    userName,
				}
				self.Contact.Friends[userName] = uf
				self.Contact.NickFriends[realName] = uf
				if realName == self.cfg.TestNickName {
					self.TestUserName = userName
					logrus.Debugf("test realname[%s] username[%s]", realName, userName)
				}
			}
		}
	}
	logrus.Debugf("webwxgetcontact get group num: %d", len(self.Contact.Groups))
	logrus.Debugf("webwxgetcontact get user friend num: %d", len(self.Contact.Friends))
	//for _,v := range self.Contact.Friends {
	//	logrus.Debugf("friend: %s", v.NickName)
	//}

	return true
}

func (self *WxWeb) webwxbatchgetcontact(usernameList []string) bool {
	urlstr := fmt.Sprintf("%s/webwxbatchgetcontact?type=ex&lang=zh_CN&pass_ticket=%s&r=%s", self.Session.BaseUri, self.Session.PassTicket, self._unixStr())
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	list := make([]map[string]interface{}, 0)
	for _, v := range usernameList {
		info := make(map[string]interface{})
		info["EncryChatRoomId"] = ""
		info["UserName"] = v
		list = append(list, info)
	}
	params["List"] = list
	params["Count"] = len(list)
	res, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("webwxbatchgetcontact _post error: %v", err)
		return false
	}
	dataJson := JsonDecode(res)
	if dataJson == nil {
		logrus.Errorf("json decode error.")
		return false
	}
	data := dataJson.(map[string]interface{})
	if data == nil {
		logrus.Errorf("webwxbatchgetcontact translate map error: %v", err)
		return false
	}
	retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
	if retCode != WX_RET_SUCCESS {
		logrus.Errorf("webwxbatchgetcontact get error retcode[%d]", retCode)
		return false
	}
	contactList := data["ContactList"].([]interface{})
	if contactList == nil {
		logrus.Errorf("webwxbatchgetcontact get contactList error")
		return false
	}
	for _, v := range contactList {
		Contact := v.(map[string]interface{})
		if Contact == nil {
			logrus.Errorf("webwxbatchgetcontact get Contact[%v] error", v)
			continue
		}
		userName := Contact["UserName"].(string)
		contactFlag := Contact["ContactFlag"].(int)
		nickName := Contact["NickName"].(string)
		// change emoji
		nickName = replaceEmoji(nickName)
		
		if strings.HasPrefix(userName, GROUP_PREFIX) {
			ug := NewUserGroup(contactFlag, nickName, userName, self)
			memberList := Contact["MemberList"].([]interface{})
			for _, v2 := range memberList {
				member := v2.(map[string]interface{})
				if member == nil {
					logrus.Errorf("webwxbatchgetcontact get member[%v] error", v2)
					continue
				}
				displayName := member["DisplayName"].(string)
				memberNickName := member["NickName"].(string)
				memberNickName = replaceEmoji(memberNickName)
				userName := member["UserName"].(string)
				gui := &GroupUserInfo{
					DisplayName: displayName,
					NickName:    memberNickName,
					UserName:    userName,
				}
				ug.MemberList[userName] = gui
				ug.NickMemberList[memberNickName] = gui
			}
			self.Contact.Groups[userName] = ug
			self.Contact.NickGroups[nickName] = ug
			logrus.Debugf("get big contact add group[%s]", nickName)
		} else {
			remarkName := Contact["RemarkName"].(string)
			alias := Contact["Alias"].(string)
			city := Contact["City"].(string)
			sex := Contact["Sex"].(int)
			verifyFlag := Contact["VerifyFlag"].(int)
			if verifyFlag == WX_FRIEND_VERIFY_FLAG_DINGYUEHAO || verifyFlag == WX_FRIEND_VERIFY_FLAG_FUWUHAO {
				continue
			}

			realName := remarkName
			if realName == "" {
				realName = nickName
			}
			realName = replaceEmoji(realName)
			_, ok := self.Contact.NickFriends[realName]
			if ok {
				realName = fmt.Sprintf("%s_$_%d", realName, time.Now().Unix())
				self.WebwxOplog(userName, realName)
				time.Sleep(time.Second)
			}

			uf := &UserFriend{
				Alias:       alias,
				City:        city,
				VerifyFlag:  verifyFlag,
				ContactFlag: contactFlag,
				NickName:    nickName,
				RemarkName:  realName,
				Sex:         sex,
				UserName:    userName,
			}
			self.Contact.Friends[userName] = uf
			self.Contact.NickFriends[realName] = uf
			if realName == self.cfg.TestNickName {
				self.TestUserName = userName
				logrus.Debugf("test realname[%s] username[%s]", realName, userName)
			}
			logrus.Debugf("get big contact add friend[%s]", nickName)
		}
	}
	return true
}

func (self *WxWeb) GroupWebwxbatchgetcontact(args ...interface{}) bool {
	urlstr := fmt.Sprintf("%s/webwxbatchgetcontact?type=ex&lang=zh_CN&pass_ticket=%s&r=%s", self.Session.BaseUri, self.Session.PassTicket, self._unixStr())
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	list := make([]map[string]interface{}, 0)
	num := 0
	for _, v := range self.Contact.Groups {
		gInfo := make(map[string]interface{})
		gInfo["EncryChatRoomId"] = ""
		gInfo["UserName"] = v.UserName
		list = append(list, gInfo)
		num++
		if num == 20 {
			params["List"] = list
			params["Count"] = len(list)
			res, err := self._post(urlstr, params, true)
			if err != nil {
				logrus.Errorf("webwxbatchgetcontact _post error: %v", err)
				return false
			}

			dataJson := JsonDecode(res)
			if dataJson == nil {
				logrus.Errorf("json decode error.")
				return false
			}
			data := dataJson.(map[string]interface{})
			if data == nil {
				logrus.Errorf("webwxbatchgetcontact translate map error: %v", err)
				return false
			}
			retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
			if retCode != WX_RET_SUCCESS {
				logrus.Errorf("webwxbatchgetcontact get error retcode[%d]", retCode)
				return false
			}

			contactList := data["ContactList"].([]interface{})
			if contactList == nil {
				logrus.Errorf("webwxbatchgetcontact get contactList error")
				return false
			}
			for _, v := range contactList {
				Contact := v.(map[string]interface{})
				if Contact == nil {
					logrus.Errorf("webwxbatchgetcontact get Contact[%v] error", v)
					continue
				}
				groupUserName := Contact["UserName"].(string)
				groupContactFlag := Contact["ContactFlag"].(int)
				groupNickName := Contact["NickName"].(string)
				groupNickName = replaceEmoji(groupNickName)
				memberList := Contact["MemberList"].([]interface{})
				for _, v2 := range memberList {
					member := v2.(map[string]interface{})
					if member == nil {
						logrus.Errorf("webwxbatchgetcontact get member[%v] error", v2)
						continue
					}
					displayName := member["DisplayName"].(string)
					nickName := member["NickName"].(string)
					nickName = replaceEmoji(nickName)
					userName := member["UserName"].(string)
					gui := &GroupUserInfo{
						DisplayName: displayName,
						NickName:    nickName,
						UserName:    userName,
					}
					gv := self.Contact.Groups[groupUserName]
					if gv == nil {
						logrus.Errorf("Contact groups have no this username[%s]", groupUserName)
						continue
					}
					gv.MemberList[userName] = gui
					gv.NickMemberList[groupNickName] = gui
					gv.NickName = groupNickName
					gv.ContactFlag = groupContactFlag
				}
				gv := self.Contact.Groups[groupUserName]
				if gv != nil {
					self.Contact.NickGroups[groupNickName] = gv
				}
			}
			// clear
			num = 0
			list = nil
		}
	}
	if num != 0 {
		params["List"] = list
		params["Count"] = len(list)
		res, err := self._post(urlstr, params, true)
		if err != nil {
			logrus.Errorf("webwxbatchgetcontact _post error: %v", err)
			return false
		}

		dataJson := JsonDecode(res)
		if dataJson == nil {
			logrus.Errorf("json decode error.")
			return false
		}
		data := dataJson.(map[string]interface{})
		if data == nil {
			logrus.Errorf("webwxbatchgetcontact translate map error: %v", err)
			return false
		}
		retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
		if retCode != WX_RET_SUCCESS {
			logrus.Errorf("webwxbatchgetcontact get error retcode[%d]", retCode)
			return false
		}

		contactList := data["ContactList"].([]interface{})
		if contactList == nil {
			logrus.Errorf("webwxbatchgetcontact get contactList error")
			return false
		}
		for _, v := range contactList {
			Contact := v.(map[string]interface{})
			if Contact == nil {
				logrus.Errorf("webwxbatchgetcontact get Contact[%v] error", v)
				continue
			}
			groupUserName := Contact["UserName"].(string)
			groupContactFlag := Contact["ContactFlag"].(int)
			groupNickName := Contact["NickName"].(string)
			groupNickName = replaceEmoji(groupNickName)
			memberList := Contact["MemberList"].([]interface{})
			for _, v2 := range memberList {
				member := v2.(map[string]interface{})
				if member == nil {
					logrus.Errorf("webwxbatchgetcontact get member[%v] error", v2)
					continue
				}
				displayName := member["DisplayName"].(string)
				nickName := member["NickName"].(string)
				nickName = replaceEmoji(nickName)
				userName := member["UserName"].(string)
				gui := &GroupUserInfo{
					DisplayName: displayName,
					NickName:    nickName,
					UserName:    userName,
				}
				gv := self.Contact.Groups[groupUserName]
				if gv == nil {
					logrus.Errorf("Contact groups have no this username[%s]", groupUserName)
					continue
				}
				gv.MemberList[userName] = gui
				gv.NickMemberList[groupNickName] = gui
				gv.NickName = groupNickName
				gv.ContactFlag = groupContactFlag
			}
			gv := self.Contact.Groups[groupUserName]
			if gv != nil {
				self.Contact.NickGroups[groupNickName] = gv
			}
		}
	}

	return true
}

func (self *WxWeb) webgetchatroommember(chatroomId string) (map[string]string, error) {
	urlstr := fmt.Sprintf("%s/webwxbatchgetcontact?type=ex&r=%s&passTicket=%s",
		self.Session.BaseUri, self._unixStr(), self.Session.PassTicket)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["Count"] = 1
	params["List"] = []map[string]string{}
	l := []map[string]string{}
	params["List"] = append(l, map[string]string{
		"UserName":   chatroomId,
		"ChatRoomId": "",
	})
	members := []string{}
	stats := make(map[string]string)
	res, err := self._post(urlstr, params, true)
	debugPrint(params)
	if err != nil {
		return stats, err
	}
	data := JsonDecode(res).(map[string]interface{})
	RoomContactList := data["ContactList"].([]interface{})[0].(map[string]interface{})["MemberList"]
	man := 0
	woman := 0
	for _, v := range RoomContactList.([]interface{}) {
		if m, ok := v.([]interface{}); ok {
			for _, s := range m {
				members = append(members, s.(map[string]interface{})["UserName"].(string))
			}
		} else {
			members = append(members, v.(map[string]interface{})["UserName"].(string))
		}
	}
	urlstr = fmt.Sprintf("%s/webwxbatchgetcontact?type=ex&r=%s&passTicket=%s",
		self.Session.BaseUri, self._unixStr(), self.Session.PassTicket)
	length := 50
	debugPrint(members)
	mnum := len(members)
	block := int(math.Ceil(float64(mnum) / float64(length)))
	k := 0
	for k < block {
		offset := k * length
		var l int
		if offset+length > mnum {
			l = mnum
		} else {
			l = offset + length
		}
		blockmembers := members[offset:l]
		params := make(map[string]interface{})
		params["BaseRequest"] = self.Session.BaseRequest
		params["Count"] = len(blockmembers)
		blockmemberslist := []map[string]string{}
		for _, g := range blockmembers {
			blockmemberslist = append(blockmemberslist, map[string]string{
				"UserName":        g,
				"EncryChatRoomId": chatroomId,
			})
		}
		params["List"] = blockmemberslist
		debugPrint(urlstr)
		debugPrint(params)
		dic, err := self._post(urlstr, params, true)
		if err == nil {
			userlist := JsonDecode(dic).(map[string]interface{})["ContactList"]
			for _, u := range userlist.([]interface{}) {
				if u.(map[string]interface{})["Sex"].(int) == 1 {
					man++
				} else if u.(map[string]interface{})["Sex"].(int) == 2 {
					woman++
				}
			}
		}
		k++
	}
	stats = map[string]string{
		"woman": strconv.Itoa(woman),
		"man":   strconv.Itoa(man),
	}
	return stats, nil
}

func (self *WxWeb) webwxsync() interface{} {
	urlstr := fmt.Sprintf("%s/webwxsync?sid=%s&skey=%s&lang=zh_CN&pass_ticket=%s",
		self.Session.BaseUri, self.Session.Sid, self.Session.SKey, self.Session.PassTicket)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["SyncKey"] = self.Session.SyncKey
	params["rr"] = ^time.Now().Unix()
	res, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("webwxsync post error: %v", err)
		return false
	}
	if res == "" {
		logrus.Errorf("webwxsync res == nil")
		return nil
	}
	dataJson := JsonDecode(res)
	if dataJson == nil {
		logrus.Errorf("webwxsync JsonDecode(res[%s]) == nil", res)
		return nil
	}
	data := dataJson.(map[string]interface{})
	retCode := data["BaseResponse"].(map[string]interface{})["Ret"].(int)
	if retCode == 0 {
		self.Session.SyncKey = data["SyncKey"].(map[string]interface{})
		self._setsynckey()
	}
	return data
}

func (self *WxWeb) Webwxverifyuser(opcode int, verifyContent, ticket, userName, nickName string) (string, bool) {
	urlstr := fmt.Sprintf("%s/webwxverifyuser?r=%s&lang=zh_CN&pass_ticket=%s",
		self.Session.BaseUri, self._unixStr(), self.Session.PassTicket)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["Opcode"] = opcode
	params["SceneList"] = []int{33}
	params["SceneListCount"] = 1
	params["VerifyContent"] = verifyContent
	params["VerifyUserList"] = []map[string]interface{}{map[string]interface{}{"Value": userName, "VerifyUserTicket": ticket}}
	params["VerifyUserListSize"] = 1
	params["skey"] = self.Session.SKey
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("webwxverifyuser error: %v", err)
		return "", false
	} else {
		logrus.Debugf("webwxverifyuser[%s] usrname[%s] get data[%s].", urlstr, userName, data)
		if CheckWebwxRetcode(data) {
			realName := nickName
			realNickName := fmt.Sprintf("%s_$_%s_$_%s", nickName, self.Session.MyNickName, time.Now().Format("20060102_15:04"))
			res, ok := self.WebwxOplog(userName, realNickName)
			if !ok {
				logrus.Errorf("nick[%s] webwxoplog realname[%s] error", nickName, realNickName)
			} else {
				if CheckWebwxRetcode(res) {
					realName = realNickName
				}
			}
			return realName, true
		}
		return "", false
	}
}

func (self *WxWeb) Webwxuploadmedia(toUserName, filePath string) (string, bool) {
	now := time.Now().Unix()
	
	self.imgMediaMutex.Lock()
	media := self.imgMediaIdMap[filePath]
	if media != nil {
		if now-media.LastTime < 3600 {
			logrus.Debugf("get filepath[%s] media from cache[%s] success.", filePath, media.MediaId)
			mediaId := media.MediaId
			self.imgMediaMutex.Unlock()
			return mediaId, true
		}
	}
	self.imgMediaMutex.Unlock()
	
	// 图片类型
	typef := z.FileType(filePath)
	logrus.Debugf("file type: %s", typef)
	if typef == "jpg" {
		typef = "jpeg"
	}

	_, file := filepath.Split(filePath)
	//urlstr := "https://file." + self.Session.BaseHost + "/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json"
	//urlstr2 := "https://file2." + self.Session.BaseHost + "/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json"
	//urlstr := "https://file.wx.qq.com/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json"
	//urlstr := "https://file.wx2.qq.com/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json"
	//https://file.wx2.qq.com/cgi-bin/mmwebwx-bin/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json
	self.Session.MediaCount += 1
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logrus.Errorf("os.stat filtpath[%s] error: %v", filePath, err)
		return "", false
	}
	fileSize := fileInfo.Size()
	uploadmediarequest := make(map[string]interface{})
	uploadmediarequest["UploadType"] = 2
	uploadmediarequest["BaseRequest"] = self.Session.BaseRequest
	uploadmediarequest["ClientMediaId"] = time.Now().UnixNano() / 1000000
	uploadmediarequest["TotalLen"] = fileSize
	uploadmediarequest["StartPos"] = 0
	uploadmediarequest["DataLen"] = fileSize
	uploadmediarequest["MediaType"] = 4
	uploadmediarequest["FromUserName"] = self.Session.User["UserName"]
	uploadmediarequest["ToUserName"] = toUserName
	uploadmediarequest["FileMd5"] = z.MD5(filePath)
	uploadmediarequestStr := JsonEncode(uploadmediarequest)

	var multipartResult bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartResult)
	multipartWriter.SetBoundary("------WebKitFormBoundaryiqkEFAw82yzyl51B")
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace("filename"), strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(file)))
	h.Set("Content-Type", "image/"+typef)
	fileWriter, err := multipartWriter.CreatePart(h)
	if err != nil {
		logrus.Error("Create form file error: ", err)
		return "", false
	}
	fh, err := os.Open(filePath)
	if err != nil {
		logrus.Error("error opening file")
		return "", false
	}
	defer fh.Close()
	_, err = io.Copy(fileWriter, fh)
	if err != nil {
		logrus.Errorf("io copy error: %v", err)
		return "", false
	}
	multipartWriter.WriteField("id", fmt.Sprintf("WU_FILE_%d", self.Session.MediaCount))
	multipartWriter.WriteField("name", file)
	multipartWriter.WriteField("type", "image/"+typef)
	multipartWriter.WriteField("lastModifiedDate", time.Now().Format("Mon Jan _2 2006 15:04:05 GMT+0800 (CST)"))
	multipartWriter.WriteField("size", strconv.Itoa(int(fileSize)))
	multipartWriter.WriteField("mediatype", "pic")
	multipartWriter.WriteField("uploadmediarequest", uploadmediarequestStr)
	for _, v := range self.cookies {
		if v.Name == "webwx_data_ticket" {
			multipartWriter.WriteField("webwx_data_ticket", v.Value)
			break
		}
	}
	multipartWriter.WriteField("pass_ticket", self.Session.PassTicket)
	multipartWriter.Close()

	urls := [2]string{
		fmt.Sprintf(`https://file.%s/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json`, self.Session.BaseHost),
		fmt.Sprintf(`https://file2.%s/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json`, self.Session.BaseHost),
	}

	for _, url := range urls {
		res, err := self._postFile(url, &multipartResult)
		if err != nil {
			logrus.Errorf("wx[%s] upload media[%s] url[%s] error: %s", self.Session.MyNickName, filePath, url, err)
			continue
		}
		data, ok := CheckWebwxResData(res)
		if !ok {
			logrus.Errorf("webwx[%s] upload media url[%s] false.", self.Session.MyNickName, url)
			continue
		}
		if !CheckWebwxRetcodeFromData(data) {
			logrus.Errorf("webwx[%s] upload media url[%s] false.", self.Session.MyNickName, url)
			continue
		}
		mediaId := data["MediaId"]
		if mediaId == nil {
			return "", false
		}
		logrus.Debugf("wx[%s] upload media[%s] success, id: %v", self.Session.MyNickName, filePath, mediaId)
		
		// for cache
		mediaIdStr := mediaId.(string)
		if media == nil {
			media = &WxWebMediaInfo{
				MediaId: mediaIdStr,
				LastTime: now,
			}
			self.imgMediaMutex.Lock()
			self.imgMediaIdMap[filePath] = media
			self.imgMediaMutex.Unlock()
		} else {
			media.MediaId = mediaIdStr
			media.LastTime = now
		}
		
		return mediaIdStr, true
	}

	return "", false

	//res, err := self._postFile(urlstr, &multipartResult)
	//if err != nil {
	//	logrus.Errorf("wx upload media[%s] error: %s", filePath, err)
	//	return "", false
	//} else {
	//	data, ok := CheckWebwxResData(res)
	//	if !ok {
	//		logrus.Errorf("webwx upload media url[%s] false.", urlstr)
	//	} else {
	//		if !CheckWebwxRetcodeFromData(data) {
	//			logrus.Errorf("webwx upload media url[%s] false.", urlstr)
	//		}
	//	}
	//	if !CheckWebwxRetcode(res) {
	//		logrus.Errorf("webwx upload media url[%s] false.", urlstr)
	//		resAgain, err := self._postFile(urlstr2, &multipartResult)
	//		if err != nil {
	//			logrus.Errorf("wx upload media[%s] error: %s", filePath, err)
	//			return "", false
	//		} else {
	//			if !CheckWebwxRetcode(resAgain) {
	//				logrus.Errorf("webwx upload media url[%s] false.", urlstr2)
	//			} else {
	//
	//			}
	//		}
	//	}
	//	if retCode != WX_RET_SUCCESS {
	//		res, err := self._postFile(urlstr2, &multipartResult)
	//		if err != nil {
	//			logrus.Errorf("wx upload media[%s] error: %s", filePath, err)
	//			return "", false
	//		} else {
	//			dataRes := JsonDecode(res)
	//			if dataRes == nil {
	//				return "", false
	//			}
	//			data := dataRes.(map[string]interface{})
	//			if data == nil {
	//				return "", false
	//			}
	//			baseResponse := data["BaseResponse"]
	//			if baseResponse == nil {
	//				return "", false
	//			}
	//			baseResponseMap := baseResponse.(map[string]interface{})
	//			if baseResponseMap == nil {
	//				return "", false
	//			}
	//			ret := baseResponseMap["Ret"]
	//			if ret == nil {
	//				return "", false
	//			}
	//			retCode := ret.(int)
	//			if retCode == WX_RET_SUCCESS {
	//				mediaId := data["MediaId"]
	//				if mediaId == nil {
	//					return "", false
	//				}
	//				logrus.Debugf("upload media[%s] success, id: %v", filePath, mediaId)
	//				return mediaId.(string), true
	//			}
	//		}
	//	} else {
	//		mediaId := data["MediaId"]
	//		if mediaId == nil {
	//			return "", false
	//		}
	//		logrus.Debugf("upload media[%s] success, id: %v", filePath, mediaId)
	//		return mediaId.(string), true
	//	}
	//}
	//return "", false
}

func (self *WxWeb) Webwxsendmsgimg(toUserName, mediaId string) bool {
	urlstr := fmt.Sprintf("%s/webwxsendmsgimg?fun=async&f=json&lang=zh_CN&pass_ticket=%s",
		self.Session.BaseUri, self.Session.PassTicket)
	clientMsgId := self._unixStr() + "0" + strconv.Itoa(rand.Int())[3:6]
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	msg := make(map[string]interface{})
	msg["Type"] = 3
	msg["MediaId"] = mediaId
	msg["FromUserName"] = self.Session.User["UserName"]
	msg["ToUserName"] = toUserName
	msg["LocalID"] = clientMsgId
	msg["ClientMsgId"] = clientMsgId
	params["Msg"] = msg
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx[%s] send mediaId[%s] toUserName[%s] error: %s", self.Session.MyNickName, mediaId, toUserName, err)
		return false
	} else {
		if CheckWebwxRetcode(data) {
			logrus.Debugf("wx[%s] send img toUserName[%s] success.", self.Session.MyNickName, toUserName)
			return true
		}
		logrus.Errorf("wx send msg img error.")
	}
	return false
}

func (self *WxWeb) Webwxsendmsg(message string, toUserName string) bool {
	urlstr := fmt.Sprintf("%s/webwxsendmsg?pass_ticket=%s", self.Session.BaseUri, self.Session.PassTicket)
	clientMsgId := self._unixStr() + "0" + strconv.Itoa(rand.Int())[3:6]
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	msg := make(map[string]interface{})
	msg["Type"] = 1
	msg["Content"] = message
	msg["FromUserName"] = self.Session.User["UserName"]
	msg["ToUserName"] = toUserName
	msg["LocalID"] = clientMsgId
	msg["ClientMsgId"] = clientMsgId
	params["Msg"] = msg
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx send msg[%s] toUserName[%s] error: %s", message, toUserName, err)
		return false
	} else {
		if CheckWebwxRetcode(data) {
			logrus.Debugf("wx[%s] send msg[%s] toUserName[%s] success.", self.Session.MyNickName, message, toUserName)
			return true
		}
		logrus.Errorf("wx[%s] send msg[%s] error.", self.Session.MyNickName, message)
	}
	return false
}

func (self *WxWeb) WebwxupdatechatroomInvitemember(groupUserName string, userNames []string) (string, bool) {
	urlstr := fmt.Sprintf("%s/webwxupdatechatroom?fun=invitemember&pass_ticket=%s",
		self.Session.BaseUri, self.Session.PassTicket)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["ChatRoomName"] = groupUserName
	params["InviteMemberList"] = strings.Join(userNames, ",")
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx invite member groupUserName[%s] error: %s", groupUserName, err)
		return "", false
	} else {
		logrus.Debugf("wx invite member groupUserName[%s] get data[%s] success.", groupUserName, data)
		return data, true
	}
}

func (self *WxWeb) WebwxupdatechatroomModTopic(groupUserName, newTopic string) bool {
	urlstr := fmt.Sprintf("%s/webwxupdatechatroom?fun=modtopic",
		self.Session.BaseUri)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["ChatRoomName"] = groupUserName
	params["NewTopic"] = newTopic
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx mod groupUserName[%s] newtopic error: %s", groupUserName, err)
		return false
	} else {
		if CheckWebwxRetcode(data) {
			logrus.Debugf("wx[%s] mod [%s] newtopic[%s] success.", self.Session.MyNickName, groupUserName, newTopic)
			return true
		}
		logrus.Errorf("wx[%s] mod [%s] topic[%s] error.", self.Session.MyNickName, groupUserName, newTopic)
	}
	return false
}

func (self *WxWeb) WebwxOplog(username string, remark string) (string, bool) {
	urlstr := fmt.Sprintf("%s/webwxoplog", self.Session.BaseUri)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["CmdId"] = 2
	params["RemarkName"] = remark
	params["UserName"] = username
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx oplog error: %v", err)
		return "", false
	} else {
		return data, true
	}
}

func (self *WxWeb) ModtopicWebwxupdatechatroom(username string, newTopic string) (string, bool) {
	urlstr := fmt.Sprintf("%s/webwxupdatechatroom?fun=modtopic", self.Session.BaseUri)
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	params["NewTopic"] = newTopic
	params["ChatRoomName"] = username
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx mod topic error: %v", err)
		return "", false
	} else {
		return data, true
	}
}

func (self *WxWeb) webwxcreatechatroom(usernameList []string, topic string) (string, bool) {
	urlstr := fmt.Sprintf("%s/webwxcreatechatroom?r=%s", self.Session.BaseUri, self._unixStr())
	params := make(map[string]interface{})
	params["BaseRequest"] = self.Session.BaseRequest
	list := make([]map[string]interface{}, 0)
	for _, v := range usernameList {
		info := make(map[string]interface{})
		info["UserName"] = v
		list = append(list, info)
	}
	params["MemberList"] = list
	params["MemberCount"] = len(list)
	params["Topic"] = topic
	data, err := self._post(urlstr, params, true)
	if err != nil {
		logrus.Errorf("wx mod topic error: %v", err)
		return "", false
	} else {
		return data, true
	}
}

func (self *WxWeb) _init() {
	logrus.SetLevel(logrus.DebugLevel)

	gCookieJar, _ := cookiejar.New(nil)
	httpclient := http.Client{
		CheckRedirect: nil,
		Jar:           gCookieJar,
	}
	self.httpClient = &httpclient
	rand.Seed(time.Now().Unix())
	str := strconv.Itoa(rand.Int())
	self.Session.DeviceId = "e" + str[2:17]
	self.Contact = NewUserContact(self)
}

func (self *WxWeb) Start() {
	if self.argv == nil {
		self.argv = &StartWxArgv{IfInvite: true, IfInviteEndExit: true, InviteMsg: self.cfg.InviteMsg}
	}

	self.startTime = time.Now().Unix()

	self.Lock()
	self.enable = true
	self.Unlock()

	logrus.Info("[*] 微信网页版 ... 开动")
	self._init()
	self._run("[*] 正在获取 uuid ... ", self.getUuid)
	self._run("[*] 正在获取 二维码 ... ", self.genQRcode)
	logrus.Infof("[*] 请使用微信扫描二维码以登录 ... uuid[%s]", self.Session.Uuid)
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
	self._run("[*] 正在登录 ... ", self.login)
	self._run("[*] 微信初始化 ... ", self.webwxinit)
	self._run("[*] 开启状态通知 ... ", self.webwxstatusnotify)
	self._run("[*] 进行同步线路测试 ... ", self.testsynccheck)
	self._run("[*] 获取好友列表 ... ", self.webwxgetcontact)
	self._run("[*] 获取群列表 ... ", self.GroupWebwxbatchgetcontact)
	//go self.Contact.InviteMembersPic()
	if self.argv.IfInvite {
		go self.Contact.InviteMembers()
	}
	if self.argv.IfClearWx {
		go self.Contact.ClearWx()
	}
	if self.argv.IfSaveRobot {
		go self.Contact.SaveRobotFriends()
	}
	if self.argv.IfCreateGroup {
		go self.Contact.CreateGroups()
	}
	
	//go self.testUploadMedia()
	//self.Contact.PrintGroupInfo()
	//self.Contact.GroupMass()

	self.Lock()
	self.ifLogin = true
	self.Unlock()
	self.wxh.Login(self.Session.Uuid)
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
			} else if selector == "4" {
				self.webwxsync()
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			} else if selector == "7" {
				self.webwxsync()
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			} else if selector == "3" {
				self.webwxsync()
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			} else {
				self.webwxsync()
				time.Sleep(WEBWX_SYNC_INTERVAL * time.Second)
			}
		}
	}
}

func (self *WxWeb) testUploadMedia() {
	mediaId, ok := self.Webwxuploadmedia(self.TestUserName, self.cfg.UploadFile)
	if ok {
		self.Webwxsendmsgimg(self.TestUserName, mediaId)
		time.Sleep(time.Hour)
		self.Webwxsendmsgimg(self.TestUserName, mediaId)
		
		self.Webwxsendmsg("xxxxx", self.TestUserName)
	}
}
