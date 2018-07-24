package main

type RedditDataImage struct {
	Source struct {
		URL string `json:"url"`
	} `json:"source"`
}

type RedditData struct {
	Title                string  `json:"title"`
	Created              float64 `json:"created"`
	Score                int     `json:"score"`
	Pinned               bool    `json:"pinned"`
	Name                 string  `json:"name"`
	NumCrossposts        int     `json:"num_crossposts"`
	NumComments          int     `json:"num_comments"`
	SubredditSubscribers int     `json:"subreddit_subscribers"`
	Preview              struct {
		Images []RedditDataImage `json:"images"`
	} `json:"preview"`
}

type RedditObject struct {
	Kind string     `json:"kind"`
	Data RedditData `json:"data"`
}

type RedditResponse struct {
	Data struct {
		Children []RedditObject `json:"children"`
		After    string         `json:"after"`
		Before   string         `json:"before"`
	} `json:"data"`
}

func (o *RedditObject) isMeme() bool {
	if o.Kind != "t3" {
		return false
	}
	if len(o.Data.Preview.Images) == 0 {
		return false
	}
	return o.Data.Preview.Images[0].Source.URL != ""
}
