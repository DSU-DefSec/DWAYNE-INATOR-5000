package main

import (
	"errors"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
)

func getCsv() (string, error) {
	/*
		teamScores, err := getStatus()
		if err != nil {
			return "", err
		}
	*/
	csvString := "Email,Alias,Team ID,Image,Score,Play Time,Elapsed Time\n"
	/*
		for _, score := range teamScores {
			csvString += score.Team.Email + ","
			csvString += score.Team.Alias + ","
			csvString += score.Team.ID + ","
			csvString += score.Image.Name + ","
			csvString += strconv.Itoa(score.Points) + ","
			csvString += formatTime(score.PlayTime) + ","
			csvString += formatTime(score.ElapsedTime) + "\n"
		}
	*/
	return csvString, nil
}

func callCiasAlex(team string) {
	// CIAS_Alex is busy at the moment...
	time.Sleep(time.Duration(rand.Intn(100)) * time.Second)

	// Get PCR inject files
	submissions, err := getSubmissions(team, 0)
	if err != nil {
		errorPrint(err)
	}
	for _, submission := range submissions {
		// Don't process invalid submissions
		if submission.Invalid {
			continue
		}

		// Mark as invalid
		submission.Invalid = true
		submission.Updated = time.Now()
		err = updateSubmission(submission)
		if err != nil {
			errorPrint(err)
			continue
		}

		// Parse filename
		splitFileName := strings.Split(submission.FileName, "_")
		if len(splitFileName) != 4 {
			errorPrint("invalid filename split for pcr:", splitFileName)
			continue
		}
		check, err := dwConf.getCheck(splitFileName[1])
		if err != nil {
			errorPrint("invalid filename check for pcr:", splitFileName[1])
			continue
		}

		// Read files and send off to PCR
		fileContent, err := os.ReadFile("submissions/" + submission.DiskFile)
		if err != nil {
			errorPrint(err)
			continue
		}
		teamObj, err := dwConf.GetTeam(team)
		if err != nil {
			errorPrint(err)
			continue
		}
		err = parsePCR(teamObj, check.FetchName(), string(fileContent))
		if err != nil {
			errorPrint(err)
		}
	}
}

func getTeamRecord(team string) (teamRecord, error) {
	record := teamRecord{}
	statusRecords, err := getStatus()
	if err != nil {
		return record, err
	}
	for _, rec := range statusRecords {
		if rec.Team == team {
			return rec, nil
		}
	}
	return record, errors.New("team not found in status")
}

func processTeamRecord(rec teamRecord, mux *sync.Mutex) {
	debugPrint("processing record for", rec.Team)
	old := teamRecord{}
	mux.Lock()
	if val, ok := recordStaging[rec.Team]; ok {
		old = val
	} else {
		debugPrint("val doesnt exist, fetching from team records")
		old, _ = getTeamRecord(rec.Team)
	}
	mux.Unlock()

	rec.Checks = sortResults(rec.Checks)

	if len(old.Checks) > 0 {
		old.Checks = sortResults(old.Checks)

		rec.ServicePoints = old.ServicePoints
		rec.InjectPoints = old.InjectPoints
		rec.RedTeamPoints = old.RedTeamPoints
		for i, c := range old.Checks {
			/*
				if i+1 >= len(rec.Checks) {
					i
				}
				fix crash when previous check size is larger
			*/
			rec.Checks[i].SlaCounter = c.SlaCounter
			rec.Checks[i].SlaViolations = c.SlaViolations
		}

	}
	slaViolations := 0
	for i := range rec.Checks {
		if rec.Checks[i].Status {
			rec.ServicePoints++
			rec.Checks[i].SlaCounter = 0
		} else {
			if rec.Checks[i].SlaCounter >= dwConf.SlaThreshold {
				debugPrint(rec.Checks[i].Name, "triggered SLA violation!")
				rec.Checks[i].SlaCounter = 0
				rec.Checks[i].SlaViolations++
			} else {
				rec.Checks[i].SlaCounter++
			}
		}
		slaViolations += rec.Checks[i].SlaViolations
	}
	rec.SlaViolations = slaViolations
	rec.Total = rec.ServicePoints - (rec.SlaViolations * dwConf.SlaPoints) + rec.InjectPoints
	mux.Lock()
	recordStaging[rec.Team] = rec
	mux.Unlock()
}

func sortResults(resList []resultEntry) []resultEntry {
	sort.SliceStable(resList, func(i, j int) bool {
		if resList[i].Ip < resList[j].Ip {
			return true
		} else if resList[i].Ip > resList[j].Ip {
			return false
		}
		return resList[i].Name < resList[j].Name
	})
	return resList
}

func sortChecks(checkList []checks.Check) []checks.Check {
	sort.SliceStable(checkList, func(i, j int) bool {
		if checkList[i].FetchIp() < checkList[j].FetchIp() {
			return true
		} else if checkList[i].FetchIp() > checkList[j].FetchIp() {
			return false
		}
		return checkList[i].FetchName() < checkList[j].FetchName()
	})
	return checkList
}

func updateVolatilePCR(pcrItem checks.PcrData) {
	// find in checks and replace it
	for i, pcr := range checks.Creds {
		if pcr.Team == pcrItem.Team && pcr.Check == pcrItem.Check {
			checks.Creds[i] = pcrItem
			return
		}
	}
	checks.Creds = append(checks.Creds, pcrItem)
}
