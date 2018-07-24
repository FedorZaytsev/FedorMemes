package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

func (p *VKWallPost) isMeme() bool {
	for _, att := range p.Attachments {
		if att.Type == "photo" || att.Type == "posted_photo" {
			return true
		}
	}
	return false
}

func (p *VKWallPost) getBestPicture() []string {
	res := []string{}
	for _, att := range p.Attachments {
		if att.Type == "photo" || att.Type == "posted_photo" {
			res = append(res, att.Photo.Photo604)
		}
	}
	return res
}

func (vk *VK) updateMemes(from time.Time) error {
	regex := regexp.MustCompile(Config.VK.SpamFilter)
	Log.Infof("updating memes until %s from publics %v", from.Format(time.RFC3339), vk.Publics)
	for public, _ := range vk.Publics {
	WallGet:
		for i := 0; ; i++ {
			resp, err := vk.sendRequest("wall.get", map[string]interface{}{
				"domain": public,
				"count":  100,
				"offset": i * 100,
			})
			if err != nil {
				return fmt.Errorf("Cannot make request for group %s. Reason %s", public, err)
			}

			posts := VKWallGet{}
			err = json.Unmarshal([]byte(resp), &posts)
			if err != nil {
				return fmt.Errorf("Cannot parse answer from wall get. Reason %s", err)
			}
			if len(posts.Response.Items) == 0 {
				break WallGet
			}
			for _, post := range posts.Response.Items {
				if time.Unix(post.Date, 0).Before(from) && post.IsPinned != 1 {
					break WallGet
				}
				if !post.isMeme() {
					continue
				}
				mem := Meme{
					MemeId:      fmt.Sprintf("%d", post.Id),
					Public:      public,
					Platform:    "vk",
					Pictures:    post.getBestPicture(),
					Description: post.Text,
					Likes:       post.Likes.Count,
					Reposts:     post.Reposts.Count,
					Views:       post.Views.Count,
					Comments:    post.Comments.Count,
					Time:        time.Unix(post.Date, 0),
				}

				if regex.MatchString(mem.Description) {
					Log.Infof("This post %v looks like adv", mem)
					continue
				}

				err = storage.AddMeme(mem)
				if err != nil {
					Log.Errorf("Cannot add meme %v to storage. Reason %s", mem, err)
				}
			}
		}
	}
	return nil
}
