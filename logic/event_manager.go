package logic

import (
	"bufio"
	"io"
	"os"
	"strings"
	"time"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/config"
	"github.com/reechou/wxrobot/wxweb"
	"golang.org/x/net/context"
)

type EventManager struct {
	sync.Mutex
	
	wxm *WxManager
	cfg *config.Config

	msgChan chan *ReceiveMsgInfo
	filters map[string][]*EventFilter
	crons   map[string][]*EventCron

	stop chan struct{}
}

func NewEventManager(wxm *WxManager, cfg *config.Config) *EventManager {
	em := &EventManager{
		wxm:     wxm,
		cfg:     cfg,
		msgChan: make(chan *ReceiveMsgInfo, EVENT_MSG_CHAN_LEN),
		filters: make(map[string][]*EventFilter),
		crons:   make(map[string][]*EventCron),
		stop:    make(chan struct{}),
	}
	em.loadFile()
	go em.Run()

	return em
}

func (self *EventManager) Stop() {
	close(self.stop)
}

func (self *EventManager) Reset() {
	close(self.stop)
	self.filters = make(map[string][]*EventFilter)
	self.crons = make(map[string][]*EventCron)
	self.stop = make(chan struct{})
}

func (self *EventManager) ReloadFile() {
	self.Lock()
	defer self.Unlock()
	
	self.Reset()
	self.loadFile()
	go self.Run()
}

func (self *EventManager) loadFile() {
	logrus.Debugf("start load event file")
	defer func() {
		logrus.Infof("load event filter: %v.", self.filters)
		logrus.Infof("load event file[%s] success.", self.cfg.WxEventFile)
	}()
	f, err := os.Open(self.cfg.WxEventFile)
	if err != nil {
		logrus.Errorf("open file[%s] error: %v", self.cfg.WxEventFile, err)
		return
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			logrus.Errorf("load event file[%s] error: %v", self.cfg.WxEventFile, err)
			break
		}
		line = strings.Replace(line, "\n", "", -1)
		argv := strings.Split(line, " ")
		if len(argv) == 0 {
			continue
		}
		switch argv[0] {
		case "filter":
			if len(argv) != 8 {
				logrus.Errorf("filter argv error: %s", line)
				continue
			}
			f := &EventFilter{
				wxm:      self.wxm,
				WeChat:   argv[1],
				Time:     argv[2],
				Event:    argv[3],
				Msg:      argv[4],
				From:     argv[5],
				FromType: argv[6],
				Do:       argv[7],
			}
			if f.Msg == EMPTY {
				f.Msg = ""
			}
			if f.From == EMPTY {
				f.From = ""
			}
			if f.FromType == EMPTY {
				f.FromType = ""
			}
			f.Init(self.stop)
			fv := self.filters[f.WeChat]
			fv = append(fv, f)
			self.filters[f.WeChat] = fv
		case "timer":
		case "cron":
			if len(argv) != 4 {
				logrus.Errorf("cron argv error: %s", line)
				continue
			}
			c := &EventCron{
				wxm:    self.wxm,
				WeChat: argv[1],
				Time:   argv[2],
				Do:     argv[3],
			}
			c.Init(self.stop)
			cv := self.crons[c.WeChat]
			cv = append(cv, c)
			self.crons[c.WeChat] = cv
		}
	}
	return
}

func (self *EventManager) Run() {
	logrus.Debugf("event manager start run.")
	logrus.Debugf("filters: %v", self.filters)
	for {
		select {
		case msg := <-self.msgChan:
			self.Lock()
			fs, ok := self.filters[msg.msg.BaseInfo.WechatNick]
			if ok {
				for _, v := range fs {
					//logrus.Debugf("find filter: %v", *v)
					select {
					case v.GetMsgChan() <- msg:
					case <-msg.ctx.Done():
						logrus.Errorf("receive msg into filter msg channal error: %v", msg.ctx.Err())
						continue
					}
				}
			}
			msg.cancel()
			self.Unlock()
		case <-self.stop:
			logrus.Infof("event manager run stopped.")
			return
		}
	}
}

func (self *EventManager) ReceiveMsg(msg *wxweb.ReceiveMsgInfo) {
	//logrus.Debugf("event manager reveive msg: %v", msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	select {
	case self.msgChan <- &ReceiveMsgInfo{msg: msg, ctx: ctx, cancel: cancel}:
	case <-ctx.Done():
		logrus.Errorf("receive msg into msg channal error: %v", ctx.Err())
		return
	case <-self.stop:
		return
	}
}
