package ext

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/reechou/wxrobot/config"
)

const (
	URI_ROBOT_ACCOUNT_ROBOT_FRIEND = "/robot/save_friends"
)

type RobotAccount struct {
	cfg    *config.Config
	client *http.Client
}

func NewRobotAccount(cfg *config.Config) *RobotAccount {
	rae := &RobotAccount{
		cfg:    cfg,
		client: &http.Client{},
	}

	return rae
}

func (we *RobotAccount) RobotAddFriends(info *RobotSaveFriendsReq) error {
	u := "http://" + we.cfg.RobotAccount.Host + URI_ROBOT_ACCOUNT_ROBOT_FRIEND
	logrus.Debugf("robot account add friends: %v url: %s", info, u)

	body, err := json.Marshal(info)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequest("POST", u, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	rsp, err := we.client.Do(httpReq)
	defer func() {
		if rsp != nil {
			rsp.Body.Close()
		}
	}()
	if err != nil {
		return err
	}
	rspBody, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	var response RobotAccountResponse
	err = json.Unmarshal(rspBody, &response)
	if err != nil {
		logrus.Errorf("robot account add friends json decode error: %s", string(rspBody))
		return err
	}
	if response.Code != 0 {
		logrus.Errorf("robot account add friends error code[%d].", response.Code)
		return fmt.Errorf("robot account add friends error code[%d].", response.Code)
	}

	return nil
}
