package main

import (
	"net/http"
	"sort"
	"strconv"
	"time"
	"fmt"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func viewStatus(c *gin.Context) {
	records, err := getStatus()
	if err != nil {
		errorOutGraceful(c, err)
	}
	ip := c.ClientIP()
	team := getUserOptional(c)
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard", gin.H{"records": records, "team": team, "ip": ip}))
}

func viewTeam(c *gin.Context) {
	team := validateTeam(c, c.Param("team"))
	limit := 10
	history, err := getTeamRecords(team.Identifier, limit)
	if err != nil {
		errorOutGraceful(c, err)
	}
	record := teamRecord{}
	if len(history) >= 1 {
		record = history[0]
	}
	c.HTML(http.StatusOK, "team.html", pageData(c, "Scoreboard", gin.H{"team": team, "record": record, "history": history, "limit": limit}))
}

func viewCheck(c *gin.Context) {
	team := validateTeam(c, c.Param("team"))
	check, err := dwConf.getCheck(c.Param("check"))
	if err != nil {
		errorOutAnnoying(c, err)
	}
	limit := 100
	results, err := getCheckResults(team, check, limit)
	if err != nil {
		errorOutGraceful(c, err)
	}
	c.HTML(http.StatusOK, "check.html", pageData(c, "check X for X", gin.H{"team": team, "check": check, "results": results, "limit": limit}))
}

func viewPCR(c *gin.Context) {
	pcrItems, err := getPCRWeb(c)
	if err != nil {
		debugPrint("viewpcr:", err)
		errorOutGraceful(c, err)
	}
	// sort pcr items based on time
	sort.SliceStable(pcrItems, func(i, j int) bool {
		return pcrItems[i].Time.After(pcrItems[j].Time)
	})

	c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "allPcrs": checks.Creds}))
}

func getPCRWeb(c *gin.Context) ([]checks.PcrData, error) {
	var err error
	team := getUser(c)
	pcrItems := []checks.PcrData{}
	if team.IsRed() {
		errorOutAnnoying(c, errors.New("no red teamers allowed in pcr"))
	} else if team.IsAdmin() {
		// debugPrint("getting all pcr items")
		pcrItems, err = getAllTeamPCRItems()
		// debugPrint("pcrItems is", pcrItems)
		if err != nil {
			errorOutGraceful(c, err)
		}
	} else {
		pcrItems, err = getPCRItems(team, checks.Web{})
	}
	return pcrItems, err
}

func submitPCR(c *gin.Context) {
	c.Request.ParseForm()
	team := getUser(c)
	var err error
	debugPrint("pcr team is", team)
	if team.IsRed() {
		errorOutAnnoying(c, errors.New("no red teamers allowed in pcr"))
	} else if team.IsAdmin() {
		userLookup := c.Request.Form.Get("username")
		if userLookup != "" {
			validUser := false
			allUsernames := []string{}
			for _, cred := range dwConf.Creds {
				allUsernames = append(allUsernames, cred.Usernames...)
			}
			for _, user := range allUsernames {
				if user == userLookup {
					validUser = true
					break
				}
			}
			if !validUser {
				pcrItems, err := getPCRWeb(c)
				if err != nil {
					debugPrint("submitpcr:", err)
					errorOutGraceful(c, err)
				}
				submiterr := errors.New("lookupPCR: invalid user: " + userLookup)
				c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "error": submiterr}))
				return
			}
			// for each team, find password for user
			pwLookup := make(map[string]map[string]string) // team --> check --> pw
			for _, t := range dwConf.Team {
				pwLookup[t.Identifier] = make(map[string]string)
				for _, b := range dwConf.Box {
					for _, c := range b.CheckList {
						if !c.FetchAnonymous() {
							tmpCredList := checks.FindCreds(t.Identifier, c.FetchName())
							debugPrint("creds is", tmpCredList.Creds)
							if val, ok := tmpCredList.Creds[userLookup]; ok {
								debugPrint("found non-defualt passsword for", userLookup+":", val)
								pwLookup[t.Identifier][c.FetchName()] = val
							} else {
								pwLookup[t.Identifier][c.FetchName()] = checks.DefaultCreds[userLookup]
							}
						}
					}
				}
			}
			c.HTML(http.StatusOK, "pcr_lookup.html", pageData(c, "pcr", gin.H{"pwLookup": pwLookup, "userLookup": userLookup}))
			return
		} else {
			team = validateTeam(c, c.Request.Form.Get("team"))
		}
	}

	submiterr := parsePCR(team, c.Request.Form.Get("check"), c.Request.Form.Get("pcr"))
	var message string
	if submiterr == nil {
		message = "PCR submitted successfully!"
	}

	pcrItems, err := getPCRWeb(c)
	if err != nil {
		debugPrint("submitpcr:", err)
		errorOutGraceful(c, err)
	}

	c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "error": submiterr, "message": message}))
}

func viewRed(c *gin.Context) {
	pcrItems, err := getPCRWeb(c)
	if err != nil {
		debugPrint("viewpcr:", err)
		errorOutGraceful(c, err)
	}
	// sort pcr items based on time
	sort.SliceStable(pcrItems, func(i, j int) bool {
		return pcrItems[i].Time.After(pcrItems[j].Time)
	})

	c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "allPcrs": checks.Creds}))
}

func getRedWeb(c *gin.Context) ([]checks.PcrData, error) {
	var err error
	team := getUser(c)
	pcrItems := []checks.PcrData{}
	if team.IsRed() {
		errorOutAnnoying(c, errors.New("no red teamers allowed in pcr"))
	} else if team.IsAdmin() {
		// debugPrint("getting all pcr items")
		pcrItems, err = getAllTeamPCRItems()
		// debugPrint("pcrItems is", pcrItems)
		if err != nil {
			errorOutGraceful(c, err)
		}
	} else {
		pcrItems, err = getPCRItems(team, checks.Web{})
	}
	return pcrItems, err
}

func submitRed(c *gin.Context) {
	c.Request.ParseForm()
	team := getUser(c)
	var err error
	debugPrint("pcr team is", team)
	if team.IsRed() {
		errorOutAnnoying(c, errors.New("no red teamers allowed in pcr"))
	} else if team.IsAdmin() {
		userLookup := c.Request.Form.Get("username")
		if userLookup != "" {
			validUser := false
			allUsernames := []string{}
			for _, cred := range dwConf.Creds {
				allUsernames = append(allUsernames, cred.Usernames...)
			}
			for _, user := range allUsernames {
				if user == userLookup {
					validUser = true
					break
				}
			}
			if !validUser {
				pcrItems, err := getPCRWeb(c)
				if err != nil {
					debugPrint("submitpcr:", err)
					errorOutGraceful(c, err)
				}
				submiterr := errors.New("lookupPCR: invalid user: " + userLookup)
				c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "error": submiterr}))
				return
			}
			// for each team, find password for user
			pwLookup := make(map[string]map[string]string) // team --> check --> pw
			for _, t := range dwConf.Team {
				pwLookup[t.Identifier] = make(map[string]string)
				for _, b := range dwConf.Box {
					for _, c := range b.CheckList {
						if !c.FetchAnonymous() {
							tmpCredList := checks.FindCreds(t.Identifier, c.FetchName())
							debugPrint("creds is", tmpCredList.Creds)
							if val, ok := tmpCredList.Creds[userLookup]; ok {
								debugPrint("found non-defualt passsword for", userLookup+":", val)
								pwLookup[t.Identifier][c.FetchName()] = val
							} else {
								pwLookup[t.Identifier][c.FetchName()] = checks.DefaultCreds[userLookup]
							}
						}
					}
				}
			}
			c.HTML(http.StatusOK, "pcr_lookup.html", pageData(c, "pcr", gin.H{"pwLookup": pwLookup, "userLookup": userLookup}))
			return
		} else {
			team = validateTeam(c, c.Request.Form.Get("team"))
		}
	}

	submiterr := parsePCR(team, c.Request.Form.Get("check"), c.Request.Form.Get("pcr"))
	var message string
	if submiterr == nil {
		message = "PCR submitted successfully!"
	}

	pcrItems, err := getPCRWeb(c)
	if err != nil {
		debugPrint("submitpcr:", err)
		errorOutGraceful(c, err)
	}

	c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "error": submiterr, "message": message}))
}

func viewInjects(c *gin.Context) {
	// view all injects and their statuses
	// global inject table
	c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{"injects": injects, "time": time.Now()}))
}

func viewInject(c *gin.Context) {
	// view individual inject
	injectId, err := strconv.Atoi(c.Param("inject"))
	if err != nil || injectId > len(injects)-1 {
		errorOutAnnoying(c, errors.New("invalid inject id"))
		return
	}

	team := getUser(c)
	var submissions []injectSubmission
	if team.IsAdmin() {
		submissions, err = groupSubmissions(dwConf, injectId)
	} else {
		submissions, err = getSubmissions(team.Identifier, injectId)
	}
	if err != nil {
		errorOutGraceful(c, err)
		return
	}

	c.HTML(http.StatusOK, "inject.html", pageData(c, "injects", gin.H{"injectId": injectId, "inject": injects[injectId], "submissions": submissions, "time": time.Now()}))
}

func submitInject(c *gin.Context) {
	team := getUser(c)
	c.Request.ParseForm()
	action := c.Request.Form.Get("action")

	injectId, err := strconv.Atoi(c.Param("inject"))
	if err != nil || injectId > len(injects)-1 {
		errorOutAnnoying(c, errors.New("invalid inject id"))
	}

	if !team.IsAdmin() && action == "" {
		file, err := c.FormFile("submission")
		if err != nil {
			c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{"error": err.Error()}))
			return

		}
		newSubmission := injectSubmission{
			Time:     time.Now(),
			Updated:  time.Now(),
			Team:     team.Identifier,
			Inject:   injectId,
			FileName: file.Filename,
			DiskFile: uuid.New().String(),
		}

		if err := c.SaveUploadedFile(file, "submissions/"+newSubmission.DiskFile); err != nil {
			c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{"error": "unable to save file"}))
			return
		}

		if err = insertSubmission(newSubmission); err != nil {
			c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{"error": "error inserting submission into db"}))
			return
		}
	} else if action == "invalid" {

		submissionId, err := strconv.Atoi(c.Request.Form.Get("submission"))
		if err != nil {
			errorOutAnnoying(c, errors.New("submissionId is not a number"))
			return
		}
		submission, err := getSubmission(team.Identifier, injectId, submissionId)
		fmt.Println(submission, err, submissionId)
		if err != nil || submission.Updated.IsZero() {
			errorOutAnnoying(c, errors.New("invalid diskfile passed to inject invalidation"))
			return
		}
		submission.Invalid = true
		submission.Updated = time.Now()
		err = updateSubmission(submission)
		if err != nil {
			errorPrint(err)
		}

	} else if action == "notify" {
		go callCiasAlex(team.Identifier)
	}

	// mark invalid:
	// mark invalid
	// grade:
	// check if admin
	// check for team/inject/grade
	viewInject(c)
}

func gradeInject(c *gin.Context) {
	injectId, err := strconv.Atoi(c.Param("inject"))
	if err != nil || injectId > len(injects)-1 {
		errorOutAnnoying(c, errors.New("invalid inject id"))
		return
	}

	team := getUser(c)
	var submission injectSubmission

	if team.IsAdmin() {
		submissionId, err := strconv.Atoi(c.Param("submission"))
		if err != nil {
			errorOutAnnoying(c, errors.New("submissionId is not a number"))
			return
		}
		submission, err = getSubmission(team.Identifier, injectId, submissionId)
		if err != nil {
			errorOutAnnoying(c, err)
			return
		}
	} else {
		errorOutAnnoying(c, errors.New("non-admin attempted grade access"))
		return
	}

	c.HTML(http.StatusOK, "grade.html", pageData(c, "grading", gin.H{"submission":submission}))

}

func submitInjectGrade(c *gin.Context) {
	team := c.PostForm("team")
	injectId, err := strconv.Atoi(c.PostForm("injectId"))
	if err != nil {
		errorOutGraceful(c, err)
		return
	}
	diskfile := c.PostForm("diskfile")

	submission, err := getSubmission(team, injectId, diskfile)
	if err != nil {
		errorOutGraceful(c, err)
		return
	}

	submission.Score, err = strconv.Atoi(c.PostForm("grade"))
	if err != nil {
		errorOutGraceful(c, err)
		return
	}
	submission.Feedback = c.PostForm("feedback")

	err = updateSubmission(submission)
	if err != nil {
		errorOutGraceful(c, err)
		return
	}

	fmt.Println("Grade: ", submission.Score, "\nFeedback: ", submission.Feedback)
	c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(injectId))
}

func viewScores(c *gin.Context) {
	if !dwConf.Verbose && !getUser(c).IsAdmin() {
		errorOutAnnoying(c, errors.New("access to score without admin or verbose mode"))
	}

	records, err := getStatus()
	if err != nil {
		errorOutGraceful(c, err)
	}
	if !dwConf.Tightlipped {
		graphScores(records)
	}

	// sort redRecords based on inverse redPoints
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Total > records[j].Total
	})

	team := getUserOptional(c)

	c.HTML(http.StatusOK, "scores.html", pageData(c, "scores", gin.H{"records": records, "team": team}))
}

func exportTeamData(c *gin.Context) {
	team := validateTeam(c, c.Param("team"))
	csvString := "time,round,service,inject,sla,"
	for _, b := range dwConf.Box {
		for _, c := range b.CheckList {
			csvString += c.FetchName() + ","
		}
	}
	csvString += "total\n"
	records, err := getTeamRecords(team.Identifier, 0)
	if err != nil {
		errorOutGraceful(c, err)
	}
	for _, r := range records {
		csvString += r.Time.In(loc).Format("03:04:05 PM") + ","
		csvString += strconv.Itoa(r.Round) + ","
		csvString += strconv.Itoa(r.ServicePoints) + ","
		csvString += strconv.Itoa(r.InjectPoints) + ","
		slaViolations := 0
		statusString := ""
		for _, c := range r.Checks {
			slaViolations += c.SlaViolations
			if c.Status {
				statusString += "up,"
			} else {
				statusString += "down,"
			}
		}
		csvString += "-" + strconv.Itoa(slaViolations*dwConf.SlaPoints) + ","
		csvString += statusString
		csvString += strconv.Itoa(r.Total) + "\n"
	}
	c.Data(200, "text/csv", []byte(csvString))
}

func pageData(c *gin.Context, title string, ginMap gin.H) gin.H {
	newGinMap := gin.H{}
	newGinMap["title"] = title
	newGinMap["user"] = getUserOptional(c)
	newGinMap["m"] = dwConf
	newGinMap["event"] = dwConf.Event
	newGinMap["loc"] = loc
	for key, value := range ginMap {
		newGinMap[key] = value
	}
	return newGinMap
}

// validateTeam tests to see if the team currently logged in
// has authorization to access the team id requested. It always
// allows if admin, and errors out if invalid user.
func validateTeam(c *gin.Context, id string) teamData {
	team := getUser(c)
	if team.Identifier == id {
		return team
	} else if team.IsAdmin() {
		if realTeam, err := dwConf.GetTeam(id); err == nil {
			return realTeam
		}
	}
	errorOutAnnoying(c, errors.New("team could not be validated"))
	return teamData{}
}

func (m *config) IsValid(team teamData, id string) bool {
	if team.Identifier == id || team.IsAdmin() {
		return true
	}
	return false
}
