package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"time"
)

type VK struct {
	ServerAddress   string
	Token           string
	VkApiVersion    string
	SpamFilter      string
	RequestTimeout  int
	LookingDuration int
	UpdateTimeout   int
	LinkFormat      string
	Publics         map[string]struct {
		Name    string
		Group   string
		GroupId int
	}

	nextTimeRequest time.Time
}

type VKWallAttachmentPhoto struct {
	Photo604 string `json:"photo_604"`
}

type VKWallAttachment struct {
	Type  string                `json:"type"`
	Photo VKWallAttachmentPhoto `json:"photo"`
}

type VKWallPost struct {
	Id          int                `json:"id"`
	Date        int64              `json:"date"`
	Text        string             `json:"text"`
	IsPinned    int                `json:"is_pinned"`
	Attachments []VKWallAttachment `json:"attachments"`

	Comments struct {
		Count int `json:"count"`
	} `json:"comments"`
	Likes struct {
		Count int `json:"count"`
	} `json:"likes"`
	Reposts struct {
		Count int `json:"count"`
	} `json:"reposts"`
	Views struct {
		Count int `json:"count"`
	} `json:"views"`
}

type VKError struct {
	Error struct {
		ErrorCode     int             `json:"error_code"`
		ErrorMsg      string          `json:"error_msg"`
		RequestParams json.RawMessage `json:"request_params"`
	} `json:"error"`
}

type VKWallGet struct {
	Response struct {
		Items []VKWallPost `json:"items"`
	} `json:"response"`
}

func (vk *VK) update() {
	until := time.Now().Add(-time.Duration(vk.LookingDuration) * time.Hour)
	err := vk.updateMemes(until)
	if err != nil {
		Log.Errorf("Cannot update memes. Reason %s", err)
	}
}

func (vk *VK) Init() error {
	ticker := time.NewTicker(time.Duration(vk.UpdateTimeout) * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				vk.update()
				err := storage.Dump()
				if err != nil {
					Log.Errorf("Cannot dump memes. Reason %s", err)
					continue
				}
				err = storage.calculateCoeffs(Config.TelegramBot.ChatId)
				if err != nil {
					Log.Errorf("Cannot calculate groups rating. Reason %s", err)
					continue
				}
			}
		}
	}()
	return nil
}

func (vk *VK) sendRequest(vkMethod string, params map[string]interface{}) (string, error) {
	return vk.sendRequestEx("GET", vkMethod, params, nil)
}

func (vk *VK) sendRequestEx(method, vkMethod string, params map[string]interface{}, body io.Reader) (string, error) {
	u, err := url.Parse(vk.ServerAddress)
	if err != nil {
		return "", fmt.Errorf("Cannot parse vk.ServerAddress. Reason %s", err)
	}
	u.Path = path.Join(u.Path, vkMethod)
	q := u.Query()
	for k, v := range params {
		q.Add(k, fmt.Sprintf("%v", v))
	}
	q.Add("access_token", Config.VK.Token)
	q.Add("v", Config.VK.VkApiVersion)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return "", fmt.Errorf("Cannot create request. Url %s. Reason %s", u.String(), err)
	}

	dump, err := httputil.DumpRequest(req, true)
	Log.Infof("dump %s %s", dump, err)

	for time.Now().Before(vk.nextTimeRequest) {
		time.Sleep(10 * time.Millisecond)
	}

	cli := &http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("Cannot perform %s request. URL %s. Reason %s", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()

	vk.nextTimeRequest = time.Now().Add(time.Duration(Config.VK.RequestTimeout) * time.Millisecond)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Unsuccessful status code %d. Status %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Cannot read body. Reason %s", err)
	}

	vkErr := VKError{}
	err = json.Unmarshal(bodyBytes, &vkErr)
	if err == nil {
		if vkErr.Error.ErrorCode != 0 {
			return "", fmt.Errorf("Error occured. Error %s", vkErr.Error.ErrorMsg)
		}
	}

	return string(bodyBytes), nil
}
