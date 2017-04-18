package models

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
)

type Robot struct {
	ID            int64  `xorm:"pk autoincr"`
	RobotWx       string `xorm:"not null default '' varchar(128) unique"`
	RobotType     int    `xorm:"not null default 0 int index"` // 0: just robot 1: robot group manager 2: robot wechat business
	IfSaveFriend  int64  `xorm:"not null default 0 int"`
	IfSaveGroup   int64  `xorm:"not null default 0 int"`
	Ip            string `xorm:"not null default '' varchar(64) index"`
	OfPort        string `xorm:"not null default '' varchar(64) index"`
	LastLoginTime int64  `xorm:"not null default 0 int"`
	Argv          string `xorm:"not null default '' varchar(2048)"`
	BaseLoginInfo string `xorm:"not null default '' varchar(2048)"`
	WebwxCookie   string `xorm:"not null default '' varchar(2048)"`
	CreatedAt     int64  `xorm:"not null default 0 int"`
	UpdatedAt     int64  `xorm:"not null default 0 int"`
}

func CreateRobot(info *Robot) error {
	if info.RobotWx == "" {
		return fmt.Errorf("wx robot wx[%s] cannot be nil.", info.RobotWx)
	}

	now := time.Now().Unix()
	info.LastLoginTime = now
	info.CreatedAt = now
	info.UpdatedAt = now

	_, err := x.Insert(info)
	if err != nil {
		logrus.Errorf("create robot error: %v", err)
		return err
	}
	logrus.Infof("create robot[%s] success.", info.RobotWx)

	return nil
}

func GetRobot(info *Robot) (bool, error) {
	has, err := x.Where("robot_wx = ?", info.RobotWx).Get(info)
	if err != nil {
		return false, err
	}
	if !has {
		logrus.Debugf("cannot find robot from robot_wx[%s]", info.RobotWx)
		return false, nil
	}
	return true, nil
}

func GetAllRobots(ip, ofPort string) ([]Robot, error) {
	var list []Robot
	err := x.Where("ip = ?", ip).And("of_port = ?", ofPort).Find(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func UpdateRobotSaveFriend(info *Robot) error {
	now := time.Now().Unix()
	info.LastLoginTime = now
	info.UpdatedAt = now
	_, err := x.Cols("if_save_friend", "ip", "of_port", "last_login_time", "updated_at").Update(info, &Robot{RobotWx: info.RobotWx})
	return err
}

func UpdateRobotSaveGroup(info *Robot) error {
	now := time.Now().Unix()
	info.LastLoginTime = now
	info.UpdatedAt = now
	_, err := x.Cols("if_save_group", "ip", "of_port", "last_login_time", "updated_at").Update(info, &Robot{RobotWx: info.RobotWx})
	return err
}

func UpdateRobotHost(info *Robot) error {
	now := time.Now().Unix()
	info.UpdatedAt = now
	_, err := x.Cols("ip", "of_port", "updated_at").Update(info, &Robot{RobotWx: info.RobotWx})
	return err
}

func UpdateRobotSession(info *Robot) error {
	now := time.Now().Unix()
	info.UpdatedAt = now
	_, err := x.Cols("base_login_info", "webwx_cookie", "updated_at").Update(info, &Robot{RobotWx: info.RobotWx})
	return err
}

func UpdateRobotArgv(info *Robot) error {
	now := time.Now().Unix()
	info.LastLoginTime = now
	info.UpdatedAt = now
	_, err := x.Cols("robot_type", "ip", "of_port", "last_login_time", "argv", "updated_at").Update(info, &Robot{RobotWx: info.RobotWx})
	return err
}
