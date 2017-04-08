package models

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
)

type RobotGroupAdd struct {
	ID        int64  `xorm:"pk autoincr"`
	RobotWx   string `xorm:"not null default '' varchar(128)"`
	GroupName string `xorm:"not null default '' varchar(128)"`
	CreatedAt int64  `xorm:"not null default 0 int"`
}

func CreateRobotGroupAdd(info *RobotGroupAdd) error {
	if info.RobotWx == "" {
		return fmt.Errorf("wx robot group add wx[%s] cannot be nil.", info.RobotWx)
	}

	now := time.Now().Unix()
	info.CreatedAt = now

	_, err := x.Insert(info)
	if err != nil {
		logrus.Errorf("create robot group add error: %v", err)
		return err
	}
	logrus.Infof("create robot[%s] group[%s] add success.", info.RobotWx, info.GroupName)

	return nil
}

func GetRobotGroupAdd(info *RobotGroupAdd) (bool, error) {
	has, err := x.Where("robot_wx = ?", info.RobotWx).And("group_name = ?", info.GroupName).Get(info)
	if err != nil {
		return false, err
	}
	if !has {
		logrus.Debugf("cannot find robot group from robot_wx[%s %s]", info.RobotWx, info.GroupName)
		return false, nil
	}
	return true, nil
}
