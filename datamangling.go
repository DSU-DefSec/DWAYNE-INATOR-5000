package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/DSU-DefSec/mew/checks"
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
			csvString += fmt.Sprintf("%d,", score.Points)
			csvString += formatTime(score.PlayTime) + ","
			csvString += formatTime(score.ElapsedTime) + "\n"
		}
	*/
	return csvString, nil
}

func getTeamRecord(team teamData) (teamRecord, error) {
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
	fmt.Println("processing record for", rec.Team.Name)
	old := teamRecord{}
	mux.Lock()
	if val, ok := recordStaging[rec.Team]; ok {
		old = val
	} else {
		fmt.Println("val doesnt exist, fetching from team records")
		old, _ = getTeamRecord(rec.Team)
	}
	mux.Unlock()

	rec.Checks = sortResults(rec.Checks)

	if len(old.Checks) > 0 {
		old.Checks = sortResults(old.Checks)

		rec.ServicePoints = old.ServicePoints
		rec.InjectPoints = old.InjectPoints
		rec.RedDetract = old.RedDetract
		rec.RedContrib = old.RedContrib
		for i, c := range old.Checks {
			rec.Checks[i].SlaCounter = c.SlaCounter
			rec.Checks[i].SlaViolations = c.SlaViolations
			rec.Checks[i].Persists = c.Persists
		}

	}
	slaViolations := 0
	for i := range rec.Checks {
		if rec.Checks[i].Status {
			rec.ServicePoints++
			rec.Checks[i].SlaCounter = 0
		} else {
			if rec.Checks[i].SlaCounter >= mewConf.SlaThreshold {
				fmt.Println(rec.Checks[i].Name, "triggered SLA violation!")
				rec.Checks[i].SlaCounter = 0
				rec.Checks[i].SlaViolations++
			} else {
				rec.Checks[i].SlaCounter++
			}
		}
		slaViolations += rec.Checks[i].SlaViolations
	}
	rec.SlaViolations = slaViolations
	rec.Total = rec.ServicePoints - (rec.SlaViolations * mewConf.SlaPoints) + rec.InjectPoints
	mux.Lock()
	recordStaging[rec.Team] = rec
	mux.Unlock()
}

func sortResults(resList []resultEntry) []resultEntry {
	sort.SliceStable(resList, func(i, j int) bool {
		if resList[i].Suffix < resList[j].Suffix {
			return true
		} else if resList[i].Suffix > resList[j].Suffix {
			return false
		}
		return resList[i].Name < resList[j].Name
	})
	return resList
}

func sortChecks(checkList []checks.Check) []checks.Check {
	sort.SliceStable(checkList, func(i, j int) bool {
		if checkList[i].FetchSuffix() < checkList[j].FetchSuffix() {
			return true
		} else if checkList[i].FetchSuffix() > checkList[j].FetchSuffix() {
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
