package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	pb "github.com/FedorZaytsev/FedorMemes"
)

type Reddit struct {
	AppId           string
	Secret          string
	Username        string
	Password        string
	UserAgent       string
	LookingDuration int
	Publics         []string

	accessToken string
	tokenType   string
}

type RedditAuthResponse struct {
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
}

func (r *Reddit) updateToken() (string, string, error) {
	cli := &http.Client{}
	body := fmt.Sprintf("grant_type=password&username=%s&password=%s", r.Username, r.Password)
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("Cannot create request to renew reddit auth. Reason %s", err)
	}
	req.Header.Set("User-Agent", r.UserAgent)
	req.SetBasicAuth(r.AppId, r.Secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := cli.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Cannot do request for renew token. Reason %s", err)
	}

	response := RedditAuthResponse{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return "", "", fmt.Errorf("Cannot decode response with new token. Reason %s", err)
	}

	if response.AccessToken == "" {
		return "", "", fmt.Errorf("Access token is empty. Reason %s", err)
	}
	if response.TokenType == "" {
		return "", "", fmt.Errorf("Token type is empty. Reason %s", err)
	}

	return response.AccessToken, response.TokenType, nil
}

func (r *Reddit) sendRequestNoCheck(method, redditPath string, params map[string]interface{}) (*http.Response, error) {
	cli := &http.Client{}
	u, err := url.Parse("https://oauth.reddit.com")
	if err != nil {
		return nil, fmt.Errorf("Cannot parse reddit site. Reason %s", err)
	}
	u.Path = path.Join(u.Path, redditPath)
	q := u.Query()
	for k, v := range params {
		q.Add(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Cannot create request for reddit. Reason %s", err)
	}
	req.Header.Set("User-Agent", r.UserAgent)
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", r.tokenType, r.accessToken))

	dump, err := httputil.DumpRequest(req, true)
	Log.Infof("dump %s %s", dump, err)

	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cannot do request for reddit. Reason %s", err)
	}

	/*dump, err = httputil.DumpResponse(resp, true)
	Log.Infof("dump response %s %s", dump, err)*/

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Wrong status code %d %s for reddit req", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

func (r *Reddit) sendRequest(method, path string, params map[string]interface{}) (*http.Response, error) {
	return r.sendRequestNoCheck(method, path, params)
}

func (r *Reddit) getMemes() ([]*pb.Meme, error) {
	until := time.Now().Add(-time.Duration(r.LookingDuration) * time.Hour)
	return r.updateMemes(until)
}

func (r *Reddit) updateMemes(from time.Time) ([]*pb.Meme, error) {
	result := []*pb.Meme{}

	for _, public := range r.Publics {
		last := ""
		count := 0
	SubredditGet:
		for {
			resp, err := r.sendRequest("GET", fmt.Sprintf("/r/%s/new.json", public), map[string]interface{}{
				"after": last,
			})
			if err != nil {
				return result, fmt.Errorf("Cannot send request for reddit. Reason %s", err)
			}
			subreddit := RedditResponse{}
			err = json.NewDecoder(resp.Body).Decode(&subreddit)
			if err != nil {
				return result, fmt.Errorf("Cannot decode subreddit answer. Reason %s", err)
			}
			if len(subreddit.Data.Children) == 0 {
				break
			}
			last = subreddit.Data.Children[len(subreddit.Data.Children)-1].Data.Name
			count += len(subreddit.Data.Children)

			for _, post := range subreddit.Data.Children {
				if time.Unix(int64(post.Data.Created), 0).Before(from) && !post.Data.Pinned {
					break SubredditGet
				}

				if !post.isMeme() {
					continue
				}
				pictures := []string{}
				for _, picture := range post.Data.Preview.Images {
					pictures = append(pictures, picture.Source.URL)
				}
				result = append(result, &pb.Meme{
					MemeId:      post.Data.Name,
					Public:      public,
					Platform:    "reddit",
					Pictures:    pictures,
					Description: post.Data.Title,
					Likes:       int32(post.Data.Score),
					Reposts:     int32(post.Data.NumCrossposts),
					Views:       int32(post.Data.SubredditSubscribers),
					Comments:    int32(post.Data.NumComments),
					Time:        int64(post.Data.Created),
				})
			}
		}
	}
	return result, nil
}

func (r *Reddit) Init() error {
	var err error
	r.accessToken, r.tokenType, err = r.updateToken()
	return err
}
