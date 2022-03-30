package checks

import (
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	GlobalTimeout, _ = time.ParseDuration("20s")
	Creds            map[uint]map[string]map[string]string
	CredLists        []CredData
)

func getCreds(teamID uint, credList string, checkName string) (string, string) {
	var usernameList CredData
	if credList != "" {
		found := false
		for _, l := range CredLists {
			if l.Name == credList {
				usernameList = l
				found = true
				break
			}
		}
		if !found {
			log.Println("Invalid cred lists for check", checkName)
			return "", ""
		}
	} else {
		usernameList = CredLists[0]
	}

	usernames := usernameList.Usernames
	rand.Seed(time.Now().UnixNano())
	if len(usernames) > 0 {
		username := usernames[rand.Intn(len(usernames))]
		if pw, ok := Creds[teamID][checkName][username]; ok {
			return username, pw
		} else {
			return username, usernameList.DefaultPw
		}
	}
	return "", ""
}

// checks for each service
type Check interface {
	Run(uint, string, chan Result)
	FetchName() string
	FetchDisplay() string
	FetchIP() string
	FetchAnonymous() bool
}

type Result struct {
	Name   string `json:"name,omitempty"`
	Box    string `json:"box,omitempty"`
	Status bool   `json:"status,omitempty"`
	IP     string `json:"ip,omitempty"`
	Error  string `json:"error,omitempty"`
	Debug  string `json:"debug,omitempty"`
}

type checkBase struct {
	Name      string // Name is the box name plus the service (ex. lunar-dns)
	Display   string // Display is the name of the service (ex. dns)
	IP        string
	CredList  string
	Port      int
	Anonymous bool
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

func (c checkBase) FetchIP() string {
	return c.IP
}

func (c checkBase) FetchAnonymous() bool {
	return c.Anonymous
}
func RunCheck(teamID uint, teamIP, boxIP, boxName string, check Check, wg *sync.WaitGroup, resChan chan Result) {
	res := make(chan Result)
	result := Result{}
	fullIP := strings.Replace(boxIP, "x", teamIP, 1)
	go check.Run(teamID, fullIP, res)
	select {
	case result = <-res:
	case <-time.After(GlobalTimeout):
		result.Error = "Timed out"
	}
	result.Name = check.FetchName()
	result.IP = fullIP
	result.Box = boxName
	resChan <- result
	wg.Done()
}

func tcpCheck(hostIP string) error {
	_, err := net.DialTimeout("tcp", hostIP, GlobalTimeout)
	return err
}

/*
func percentChangedCreds() map[string]float {
	// get all usernames
	// for each team, see which % of creds exist in pcritems
}
*/
