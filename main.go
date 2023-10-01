package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type InstagramStories struct {
	ReelsMedia []struct {
		Depth int
		Items []struct {
			ID            string `json:"id"`
			ImageVersion2 struct {
				Candiates []struct {
					URL string `json:"url"`
				} `json:"candidates"`
			} `json:"image_versions2"`
			StoryBlockStickers []struct {
				BlockSticker struct {
					StickerData struct {
						IGMentions struct {
							AccountID     string `json:"account_id"`
							Username      string `json:"username"`
							FullName      string `json:"full_name"`
							ProfilePicURL string `json:"profile_pic_url"`
						} `json:"ig_mention"`
					} `json:"sticker_data"`
				} `json:"bloks_sticker"`
			} `json:"story_bloks_stickers"`
		} `json:"items"`
		User struct {
			Username string `json:"username"`
			PK       string `json:"pk"`
		} `json:"user"`
	} `json:"reels_media"`
}

var db *sql.DB

func getreel(userids []string) InstagramStories {
	reelsurl := "https://www.instagram.com/api/v1/feed/reels_media/?reel_ids=" + userids[0]
	for i := 0; i < len(userids); i++ {
		reelsurl += "&reel_ids=" + userids[i]
	}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", reelsurl, nil)
	req.Header.Set("Cookie", `REPLACE WITH BROWSER COOKIES`)
	req.Header.Set("User-Agent", `Mozilla/5.0 (Linux; Android 10; SM-G981B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.162 Mobile Safari/537.36`)
	req.Header.Set("X-Ig-App-Id", `1217981644879628`)
	req.Header.Set("Sec-Fetch-Site", `same-origin`)
	req.Header.Set("Sec-Fetch-Mode", `cors`)
	req.Header.Set("Csrftoken", `0qs2jFAMqzaAqrSR89peZBi2J5PwNYeA`)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var instastories InstagramStories
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &instastories); err != nil {
		log.Println(string(data))
		panic(err)
	}
	return instastories
}

func downloadimgs(imgurl, filname string) {
	resp, err := http.Get(imgurl)
	if err != nil {
		log.Println(imgurl, filname)
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	f, err := os.Create(os.Args[1] + filname + ".jpg")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		log.Println(imgurl, filname)
		panic(err)
	}
}

func saveinfo(instastories InstagramStories) {
	currentTime := time.Now()
	formatTime := currentTime.Format("2006-01-02 15:04:05")
	for _, reelmedia := range instastories.ReelsMedia {
		for _, item := range reelmedia.Items {
			for _, bs := range item.StoryBlockStickers {
				muserid := bs.BlockSticker.StickerData.IGMentions.AccountID
				musername := bs.BlockSticker.StickerData.IGMentions.Username
				mprofilepic := bs.BlockSticker.StickerData.IGMentions.ProfilePicURL
				_, err := db.Exec("INSERT INTO instagraph SELECT ?, ?, ?, ?, ?, ?  WHERE ? NOT IN (SELECT mentionid FROM instagraph);",
					reelmedia.User.Username, reelmedia.User.PK, musername, muserid, reelmedia.Depth, formatTime, muserid)
				if err != nil {
					log.Println(reelmedia.User.Username, reelmedia.User.PK, musername, muserid, reelmedia.Depth, formatTime)
					panic(err)
				}
				_, err = db.Exec("INSERT OR IGNORE INTO queue SELECT ?, ?, ? WHERE ? NOT IN (SELECT userid FROM instagraph)",
					muserid, musername, reelmedia.Depth, muserid)
				if err != nil {
					log.Println(muserid, musername, reelmedia.Depth)
					panic(err)
				}
				downloadimgs(mprofilepic, musername+"_profilepic")
				log.Println(reelmedia.User.Username, reelmedia.User.PK, musername, muserid, reelmedia.Depth, formatTime)
			}
			downloadimgs(item.ImageVersion2.Candiates[0].URL, reelmedia.User.Username+"_story_"+item.ID)
		}
	}
}

func main() {
	log.Println("Started")

	db, _ = sql.Open("sqlite3", "db/instadata.sqlite")

	for {
		var userids, usernames []string
		depths := make(map[string]int)

		rows, err := db.Query("SELECT username, userid, depth FROM queue LIMIT 5")
		if err != nil {
			panic(err)
		}
		for rows.Next() {
			var username, userid string
			var depth int
			rows.Scan(&username, &userid, &depth)
			userids = append(userids, userid)
			usernames = append(usernames, username)
			depths[username] = depth
		}

		instastories := getreel(userids)
		log.Println(usernames)

		for i := 0; i < len(instastories.ReelsMedia); i++ {
			instastories.ReelsMedia[i].Depth = depths[instastories.ReelsMedia[i].User.Username] + 1
		}
		saveinfo(instastories)

		for i := 0; i < len(userids); i++ {
			_, err := db.Exec("DELETE FROM queue WHERE userid=?", userids[i])
			if err != nil {
				panic(err)
			}
		}

		log.Println("sleeping")
		time.Sleep(18 * time.Second)
	}
}
