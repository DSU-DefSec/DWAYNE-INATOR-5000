package checks

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	GlobalTimeout, _ = time.ParseDuration("5s")
	Creds            = make(map[string]string)
	CredLists        = make(map[string][]string)
)

// checks for each service
type Check interface {
	Run(string, chan Result)
	FetchName() string
}

type Result struct {
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
	Red  bool `json:"red,omitempty"`
	Error  string `json:"error,omitempty"`
	Debug  string `json:"debug,omitempty"`
}

type checkBase struct {
	Name      string
	Suffix    string
	CredLists []string
}

type CredData struct {
	Name      string
	Usernames []string
	DefaultPw string
}

func (c checkBase) FetchName() string {
	return c.Name
}

func getCreds(credLists []string) (string, string) {
	allUsernames := []string{}
	rand.Seed(time.Now().UnixNano())
	for _, l := range credLists {
		allUsernames = append(allUsernames, CredLists[l]...)
	}
	username := allUsernames[rand.Intn(len(allUsernames))]
	return username, Creds[username]
}

func RunCheck(teamPrefix string, check Check, wg *sync.WaitGroup, resChan chan Result) {
	res := make(chan Result)
	result := Result{}
	go check.Run(teamPrefix, res)
	select {
	case result = <-res:
	case <-time.After(GlobalTimeout):
		result.Status = false
		result.Error = "timed out"
		result.Debug = "check data " + fmt.Sprint(check)
	}
	result.Name = check.FetchName()
	resChan <- result
	wg.Done()
}
