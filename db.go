package main

import (
	"strings"
	"sync"
	"time"
	//"log"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
)

const (
	EMPTY = iota
	SUBMITTED
	GRADED
)

var (
	recordsStaging = []TeamRecord{}
)

type ResultEntry struct {
	ID           uint
	Time         time.Time
	TeamID       uint
	TeamRecordID uint
	Round        int
	RoundCount   int
	// Total points check has earned
	Points int

	// Uptime is only used in the uptime view
	Uptime int `gorm:"-"`

	checks.Result
}

type TeamRecord struct {
	ID               uint
	Time             time.Time
	TeamID           uint
	Team             TeamData
	Round            int
	Results          []ResultEntry
	ResultsMap       map[string]ResultEntry `gorm:"-"`
	RedTeamPoints    int
	ServicePoints    int
	InjectPoints     int
	SlaViolations    int
	ManualAdjustment int

	// Field must be calculated before displaying.
	// We don't want to hardcode weights.
	Total int

	// Others persisting on us.
	Persists      []Persist
	PointsLost    int
	PointsStolen  int
	PersistPoints int
}

type Persist struct {
	ID           uint
	Round        int
	Box          string
	TeamID       uint
	Team         TeamData
	TeamRecordID uint
	OffenderID   uint
	Offender     TeamData
}

type SLA struct {
	Time       time.Time
	TeamID     uint   `gorm:"primaryKey"`
	Reason     string `gorm:"primaryKey"`
	Counter    int
	Violations int
}

type TeamData struct {
	ID           uint
	Name, IP, Pw string
	Token        string
}

type InjectSubmission struct {
	ID       uint
	Time     time.Time `json:"time"`
	Updated  time.Time `json:"updated"`
	TeamID   uint
	Team     TeamData
	InjectID uint   `json:"inject"`
	FileName string `json:"filename"`
	DiskFile string `json:"diskfile"`
	Invalid  bool   `json:"invalid"`
	Graded   bool
	Score    int `json:"score"`
	Content  string
	Feedback string `json:"feedback"`
}

type Inject struct {
	ID          uint
	Time        time.Time `json:"time"`
	Due         time.Time `json:"due"`
	Closes      time.Time `json:"closes"`
	Submissions []InjectSubmission
	Title       string `json:"title"`
	Body        string `json:"body"`
	File        string `json:"file"`
	Points      int    `json:"points"`
	Status      int    `json:"status"`
}

// add start time
func (i *Inject) OpenTime() time.Time {
	if i.Time.IsZero() {
		return startTime
	}
	return startTime.Add(i.Time.Sub(ZeroTime)).In(loc)
}

func (i *Inject) DueTime() time.Time {
	return startTime.Add(i.Due.Sub(ZeroTime)).In(loc)
}

func (i *Inject) CloseTime() time.Time {
	return startTime.Add(i.Closes.Sub(ZeroTime)).In(loc)
}

type CredentialTable struct {
	Creds map[uint]map[string]map[string]string
	Mutex *sync.Mutex
}

// PCRs are stored in memory as a map, constructed from a series
// of PCRs that contain changes. Kind of like a git repo.
//
// The structure goes like:
//
//	creds[team][check][username]
//
// Thus, it serves to create a lookup table state for each team.
func constructPCRState() {
	ct.Mutex.Lock()

	// nuke current state
	ct.Creds = make(map[uint]map[string]map[string]string)

	// get all pcr entries
	var submissions []InjectSubmission
	// password change inject is always inject 1
	res := db.Where("inject_id = 1").Find(&submissions)
	if res.Error != nil {
		errorPrint(res.Error)
		return
	}

	// for each pcr entry
	for _, submission := range submissions {
		if submission.Feedback != "" {
			continue
		}

		if submission.Invalid && !submission.Graded {
			continue
		}

		if !submission.Invalid {
			submission.Invalid = true
		}

		fileName := strings.Split(submission.FileName, "_pcr_")
		if len(fileName) != 4 {
			submission.Feedback = "file name is improperly formatted"
			db.Save(&submission)
			continue
		}

		check, err := dwConf.getCheck(fileName[1])
		if err != nil {
			submission.Feedback = err.Error()
			db.Save(&submission)
			continue
		}

		content := readInject(submission)

		// parse and add to table
		usernames := []string{}
		passwords := []string{}
		splitPcr := strings.Split(content, "\n")
		if len(splitPcr) == 0 || splitPcr[0] == "" {
			submission.Feedback = "input empty"
			db.Save(&submission)
			continue
		}

		if len(splitPcr) > 10000 {
			submission.Feedback = "input too large (over 10000 lines)"
			db.Save(&submission)
			continue
		}

		allUsernames := []string{}
		for _, cred := range dwConf.Creds {
			allUsernames = append(allUsernames, cred.Usernames...)
		}

		empty := true
		for _, p := range splitPcr {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			splitItem := strings.Split(p, ",")
			if len(splitItem) != 2 {
				submission.Feedback = "username was an invalid format: " + p
				db.Save(&submission)
				break
			}

			if splitItem[1] == "" {
				continue
			}

			empty = false

			if splitItem[0] == "all" {
				for _, user := range allUsernames {
					usernames = append(usernames, user)
					passwords = append(passwords, splitItem[1])
				}
			} else {
				validUser := false
				for _, user := range allUsernames {
					if user == splitItem[0] {
						validUser = true
						break
					}
				}

				if !validUser {
					continue
				}

				usernames = append(usernames, splitItem[0])
				passwords = append(passwords, splitItem[1])
			}
		}

		if submission.Feedback != "" {
			continue
		}

		if empty {
			submission.Feedback = "input empty"
			db.Save(&submission)
			continue
		}

		if !submission.Graded {
			submission.Updated = time.Now()
			submission.Graded = true
			db.Save(&submission)
		}

		// add creds to pcrItem
		for i, u := range usernames {
			if _, ok := ct.Creds[submission.TeamID]; !ok {
				ct.Creds[submission.TeamID] = make(map[string]map[string]string)
			}
			if _, ok := ct.Creds[submission.TeamID][check.FetchName()]; !ok {
				ct.Creds[submission.TeamID][check.FetchName()] = make(map[string]string)
			}
			ct.Creds[submission.TeamID][check.FetchName()][u] = passwords[i]
		}

	}

	checks.Creds = ct.Creds
	ct.Mutex.Unlock()
}
