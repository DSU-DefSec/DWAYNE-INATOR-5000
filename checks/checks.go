package checks

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	GlobalTimeout, _ = time.ParseDuration("5s")
	Creds            = make(map[string]string)
	DefaultCredList  = []string{}
	CredLists        = make(map[string][]string)
)

// checks for each service
type Check interface {
	Run(string, chan Result)
	FetchName() string
	FetchDisplay() string
	FetchSuffix() string
}

type Result struct {
	Name    string   `json:"name,omitempty"`
	Box 	string`json:"box,omitempty"`
	Status  bool     `json:"status,omitempty"`
	Suffix  int      `json:"suffix,omitempty"`
	Persists map[string][]string `json:"persists,omitempty"`
	Error   string   `json:"error,omitempty"`
	Debug   string   `json:"debug,omitempty"`
}

type checkBase struct {
	Name      string
	Display   string
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

func (c checkBase) FetchDisplay() string {
	return c.Display
}

func (c checkBase) FetchSuffix() string {
	return c.Suffix
}

func getCreds(credLists []string) (string, string, error) {
	allUsernames := []string{}
	rand.Seed(time.Now().UnixNano())
	if len(credLists) > 0 {
		for _, l := range credLists {
			allUsernames = append(allUsernames, CredLists[l]...)
		}
	} else {
		allUsernames = DefaultCredList
	}
	if len(allUsernames) > 0 {
		username := allUsernames[rand.Intn(len(allUsernames))]
		return username, Creds[username], nil
	}
	return "", "", errors.New("getCreds: empty credlist")
}

func RunCheck(teamPrefix, boxSuffix, boxName string, check Check, wg *sync.WaitGroup, resChan chan Result) {
	res := make(chan Result)
	result := Result{}
	go check.Run(teamPrefix+boxSuffix, res)
	select {
	case result = <-res:
	case <-time.After(GlobalTimeout):
		result.Status = false
		result.Error = "timed out"
		result.Debug = "check data " + fmt.Sprint(check)
	}
	result.Name = check.FetchName()
	result.Suffix, _ = strconv.Atoi(boxSuffix)
	result.Box = boxName
	resChan <- result
	wg.Done()
}

func tcpCheck(hostIp string) error {
	_, err := net.DialTimeout("tcp", hostIp, GlobalTimeout)
	return err
}

func (r Result) IsHacked() bool {
	if _, ok := r.Persists[r.Box]; ok {
		return true
	}
	return false
}
