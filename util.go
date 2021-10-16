package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
	"github.com/gin-gonic/gin"
)

func errorOut(c *gin.Context, err error) {
	errorPrint("error:", err)
	c.JSON(400, gin.H{"error": "Invalid request."})
	c.Abort()
}

func errorOutGraceful(c *gin.Context, err error) {
	errorPrint("error:", err)
	c.Redirect(http.StatusSeeOther, "/")
	c.Abort()
}

func errorOutAnnoying(c *gin.Context, err error) {
	errorPrint("error:", err)
	c.Redirect(http.StatusSeeOther, "/forbidden")
	c.Abort()
}

func parseTime(timeStr string) time.Time {
	timeStr += " " + locString
	parsedTime, err := time.Parse("01/02/06 3:04 MST", timeStr)
	if err != nil {
		errorPrint("time parsing failed,", timeStr, "did not parse correctly:", err.Error())
	}
	return parsedTime
}

func formatTime(dur time.Duration) string {
	durSeconds := dur.Microseconds() / 1000000
	seconds := durSeconds % 60
	durSeconds -= seconds
	minutes := (durSeconds % (60 * 60)) / 60
	durSeconds -= minutes * 60
	hours := durSeconds / (60 * 60)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

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

func sortResults(resList []ResultEntry) []ResultEntry {
	sort.SliceStable(resList, func(i, j int) bool {
		if resList[i].IP < resList[j].IP {
			return true
		} else if resList[i].IP > resList[j].IP {
			return false
		}
		return resList[i].Name < resList[j].Name
	})
	return resList
}

func sortChecks(checkList []checks.Check) []checks.Check {
	sort.SliceStable(checkList, func(i, j int) bool {
		if checkList[i].FetchIP() < checkList[j].FetchIP() {
			return true
		} else if checkList[i].FetchIP() > checkList[j].FetchIP() {
			return false
		}
		return checkList[i].FetchName() < checkList[j].FetchName()
	})
	return checkList
}

func validateString(input string) bool {
	if input == "" {
		return false
	}
	validationString := `^[a-zA-Z0-9-_]+$`
	inputValidation := regexp.MustCompile(validationString)
	return inputValidation.MatchString(input)
}

func (t TeamData) IsValid() bool {
	return t.Name != ""
}

func (m *config) getCheck(checkName string) (checks.Check, error) {
	for _, box := range m.Box {
		for _, check := range box.CheckList {
			if check.FetchName() == checkName {
				return check, nil
			}
		}
	}
	return checks.Web{}, errors.New("check not found")
}

func calculateScoreTotal(rec TeamRecord) int {
	total := rec.ServicePoints + rec.InjectPoints
	total -= rec.RedTeamPoints + (rec.SlaViolations * dwConf.SlaPoints)
	if dwConf.Persists {
		total += rec.PointsStolen + rec.PersistPoints
		total -= rec.PointsLost
	}
	total += rec.ManualAdjustment
	return total
}

func readInject(inj InjectSubmission) string {
	content, err := os.ReadFile("submissions/" + inj.DiskFile)
	if err != nil {
		errorPrint(err)
		return ""
	}
	return string(content)
}

func boxFromIP(ip string) (TeamData, string, error) {
	for _, box := range dwConf.Box {
		for _, t := range dwConf.Team {
			if ip == strings.Replace(box.IP, "x", t.IP, 1) {
				return t, box.Name, nil
			}
		}
	}
	return TeamData{}, "", errors.New("box not found")
}

func tokenToTeam(token string) (TeamData, error) {
	for _, t := range dwConf.Team {
		if t.Token == token {
			return t, nil
		}
	}
	return TeamData{}, errors.New("invalid token")
}

func (m *config) GetTeam(teamID uint) (TeamData, error) {
	for _, team := range m.Team {
		if team.ID == teamID {
			return team, nil
		}
	}
	return TeamData{}, errors.New("team not found")
}

func oneOfN(points, parties int) int {
	return points / parties
}
