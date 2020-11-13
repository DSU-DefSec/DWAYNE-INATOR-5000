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
	DefaultCreds     = make(map[string]string)
	Creds            = []PcrData{}
	DefaultCredList  = []string{}
	CredLists        = make(map[string][]string)
	Colors           = make(map[string]string) // this sucks lol
)

// checks for each service
type Check interface {
	Run(string, string, chan Result)
	FetchName() string
	FetchDisplay() string
	FetchSuffix() string
	FetchAnonymous() bool
}

type Result struct {
	Name     string              `json:"name,omitempty"`
	Box      string              `json:"box,omitempty"`
	Status   bool                `json:"status,omitempty"`
	Suffix   int                 `json:"suffix,omitempty"`
	Persists map[string][]string `json:"persists,omitempty"`
	Error    string              `json:"error,omitempty"`
	Debug    string              `json:"debug,omitempty"`
}

type checkBase struct {
	Name      string
	Display   string
	Suffix    string
	CredLists []string
	Anonymous bool
}

type CredData struct {
	Name      string
	Usernames []string
	DefaultPw string
}

type PcrData struct {
	Time  time.Time
	Team  string
	Check string
	Creds map[string]string
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

func (c checkBase) FetchAnonymous() bool {
	return c.Anonymous
}

func getCreds(credLists []string, teamIdentifier, checkName string) (string, string, error) {
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
		credItem := FindCreds(teamIdentifier, checkName)
		if credItem.Team == "" {
			return username, DefaultCreds[username], nil
		}
		if pw, ok := credItem.Creds[username]; !ok {
			return username, DefaultCreds[username], nil
		} else {
			return username, pw, nil
		}
	}
	return "", "", errors.New("getCreds: empty credlist")
}

func FindCreds(teamName, checkName string) PcrData {
	for i, pcr := range Creds {
		if pcr.Team == teamName && pcr.Check == checkName {
			return Creds[i]
		}
	}
	return PcrData{}
}

func RunCheck(teamName, teamPrefix, boxSuffix, boxName string, check Check, wg *sync.WaitGroup, resChan chan Result) {
	res := make(chan Result)
	result := Result{}
	go check.Run(teamName, teamPrefix+boxSuffix, res)
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

/*
func percentChangedCreds() map[string]float {
	// get all usernames
	// for each team, see which % of creds exist in pcritems
}
*/

func (r Result) IsHacked() bool {
	if _, ok := r.Persists[r.Box]; ok {
		return true
	}
	return false
}

func (r Result) GetColors() []string {
	colors := []string{}
	if val, ok := r.Persists[r.Box]; ok {
		for _, team := range val {
			colors = append(colors, Colors[team])
		}
	}
	return colors
}
