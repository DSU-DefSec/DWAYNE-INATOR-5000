package checks

import (
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	GlobalTimeout, _ = time.ParseDuration("20s")
	DefaultCreds     = make(map[string]string)
	Creds            = []PcrData{}
	DefaultCredList  = []string{}
	CredLists        = make(map[string][]string)
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
	Name   string `json:"name,omitempty"`
	Box    string `json:"box,omitempty"`
	Status bool   `json:"status,omitempty"`
	Suffix int    `json:"suffix,omitempty"`
	Error  string `json:"error,omitempty"`
	Debug  string `json:"debug,omitempty"`
}

type checkBase struct {
	Name      string // Name is the box name plus the service (ex. lunar-dns)
	Display   string // Display is the name of the service (ex. dns)
	Suffix    string
	CredLists []string
    Port int
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

func getCreds(credLists []string, teamIdentifier, checkName string) (string, string) {
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
			return username, DefaultCreds[username]
		}
		if pw, ok := credItem.Creds[username]; !ok {
			return username, DefaultCreds[username]
		} else {
			return username, pw
		}
	}
	return "", ""
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
		result.Error = "timed out"
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
