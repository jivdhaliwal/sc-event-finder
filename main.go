package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
)

type SCUser struct {
	ID       int    `json:"id"`
	UserName string `json:"username"`
}

type SCUsersCollection struct {
	Collection []SCUser `json:"collection"`
}

type SKArtistsResult struct {
	ResultsPage struct {
		Status       string `json:"status"`
		PerPage      int    `json:"perPage"`
		Page         int    `json:"page"`
		TotalEntries int    `json:"totalEntries"`
		Results      struct {
			Artist []SKArtist `json:"artist"`
		} `json:"results"`
	} `json:"resultsPage"`
}

type SKArtist struct {
	ID          int                  `json:"id"`
	DisplayName string               `json:"displayName"`
	Identifier  []SKArtistIdentifier `json:"identifier"`
}

type SKArtistIdentifier struct {
	EventsHref string `json:"eventsHref"`
}

type SKEventsResult struct {
	ResultsPage struct {
		Status       string `json:"status"`
		PerPage      int    `json:"perPage"`
		Page         int    `json:"page"`
		TotalEntries int    `json:"totalEntries"`
		Results      struct {
			Event []SKEvent `json:"event"`
		} `json:"results"`
	} `json:"resultsPage"`
}

type SKEvent struct {
	ID          int    `json:"id"`
	DisplayName string `json:"displayName"`
	Start       struct {
		Date string `json:"date"`
	} `json:"start"`
	Location struct {
		City string `json:"city"`
	} `json:"location"`
	Venue struct {
		DisplayName string `json:"displayName"`
	} `json:"venue"`
}

func DecodeUser(r io.Reader) (x *SCUser, err error) {
	x = new(SCUser)
	err = json.NewDecoder(r).Decode(x)
	return
}

func DecodeFollowingUsers(r io.Reader) (x *SCUsersCollection, err error) {
	x = new(SCUsersCollection)
	err = json.NewDecoder(r).Decode(x)
	return
}

func DecodeSKArtists(r io.Reader) (x *SKArtistsResult, err error) {
	x = new(SKArtistsResult)
	err = json.NewDecoder(r).Decode(x)
	return
}

func DecodeSKEvents(r io.Reader) (x *SKEventsResult, err error) {
	x = new(SKEventsResult)
	err = json.NewDecoder(r).Decode(x)
	return
}

func getSCUser(userName string) (*SCUser, error) {
	resp, err := http.Get(fmt.Sprintf("http://api.soundcloud.com/resolve?url=http://soundcloud.com/%v&client_id=%v", userName, os.Getenv("SOUNDCLOUD_API_KEY")))
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return new(SCUser), errors.New("Soundcloud user not found")
	}

	user, err := DecodeUser(resp.Body)
	return user, err
}

func getSCFollowingsList(userID int) *SCUsersCollection {
	followingResp, _ := http.Get(fmt.Sprintf("http://api.soundcloud.com/users/%v/followings?client_id=%v", userID, os.Getenv("SOUNDCLOUD_API_KEY")))
	defer followingResp.Body.Close()

	followingUsers, _ := DecodeFollowingUsers(followingResp.Body)
	return followingUsers
}

func getSKArtists(artistName string) *SKArtistsResult {
	artistResp, _ := http.Get(fmt.Sprintf("http://api.songkick.com/api/3.0/search/artists.json?query=%v&apikey=%v", url.QueryEscape(artistName), os.Getenv("SONGKICK_API_KEY")))
	defer artistResp.Body.Close()
	skArtistResults, _ := DecodeSKArtists(artistResp.Body)
	return skArtistResults
}

func getSKEvents(skArtist SKArtist) *SKEventsResult {
	artistEventsResp, _ := http.Get(fmt.Sprintf("%v?apikey=%v", skArtist.Identifier[0].EventsHref, os.Getenv("SONGKICK_API_KEY")))
	defer artistEventsResp.Body.Close()
	skEvents, _ := DecodeSKEvents(artistEventsResp.Body)
	return skEvents
}

func getEvents(followingsList []SCUser) <-chan SKEvent {
	eventChannel := make(chan SKEvent)
	var wg sync.WaitGroup
	wg.Add(len(followingsList))

	userEvents := func(userName string, ec chan<- SKEvent) {
		defer wg.Done()

		skArtistResults := getSKArtists(userName)
		if skArtistResults.ResultsPage.TotalEntries > 0 {
			skArtist := skArtistResults.ResultsPage.Results.Artist[0]

			if len(skArtist.Identifier) > 0 {
				skEvents := getSKEvents(skArtist)

				if skEvents.ResultsPage.TotalEntries > 0 {
					ec <- skEvents.ResultsPage.Results.Event[0]
				}
			}
		}
	}

	for _, followingUser := range followingsList {
		go userEvents(followingUser.UserName, eventChannel)
	}

	go func() {
		wg.Wait()
		close(eventChannel)
	}()

	return eventChannel
}

func main() {
	if len(os.Getenv("SONGKICK_API_KEY")) == 0 {
		fmt.Println("Songkick API Key missing")
		return
	}

	if len(os.Getenv("SOUNDCLOUD_API_KEY")) == 0 {
		fmt.Println("Soundcloud API Key missing")
		return
	}

	userName := os.Args[1]

	user, err := getSCUser(userName)

	if err != nil {
		fmt.Println(err)
		return
	}

	followingUsers := getSCFollowingsList(user.ID)

	eventsList := getEvents(followingUsers.Collection)

	for event := range eventsList {
		fmt.Printf("%v, %v\n", event.DisplayName, event.Location.City)
	}
}
