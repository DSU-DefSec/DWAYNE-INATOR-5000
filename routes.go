package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"gorm.io/gorm/clause"
)

var (
	cachedStatus []TeamRecord
	cachedRound  int
	// In-memory cache of persists for this round. map[team.ID][box.Name][]offender.ID
	persistHits map[uint]map[string][]uint
)

func viewStatus(c *gin.Context) {
	if roundNumber != cachedRound {
		var records []TeamRecord
		res := db.Limit(len(dwConf.Team)).Preload(clause.Associations).Order("time desc").Find(&records)
		if res.Error != nil {
			errorOutGraceful(c, res.Error)
		}

		// Sort results for viewing.
		for i, rec := range records {
			records[i].Results = sortResults(rec.Results)
		}

		// Sort by team ID.
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].TeamID < records[j].TeamID
		})

		cachedStatus = records
		cachedRound = roundNumber
	}
	ip := c.ClientIP()
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard", gin.H{"records": cachedStatus, "ip": ip, "round": roundNumber}))
}

func viewTeam(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("team"))
	if err != nil {
		errorOutAnnoying(c, errors.New("invalid team id: "+c.Param("team")))
		return
	}
	team := validateTeam(c, uint(id))

	var records []TeamRecord
	res := db.Limit(10).Preload("Results").Order("time desc").Find(&records, "team_id = ?", team.ID)
	if res.Error != nil {
		errorOutGraceful(c, res.Error)
		return
	}

	// Sort all the Results...
	for i := range records {
		records[i].Results = sortResults(records[i].Results)
	}

	c.HTML(http.StatusOK, "team.html", pageData(c, "Scoreboard", gin.H{"team": team, "records": records}))
}

func viewCheck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("team"))
	if err != nil {
		errorOutAnnoying(c, errors.New("invalid team id: "+c.Param("team")))
		return
	}
	team := validateTeam(c, uint(id))

	check, err := dwConf.getCheck(c.Param("check"))
	if err != nil {
		errorOutAnnoying(c, err)
	}

	var sla SLA
	res := db.Limit(1).Find(&sla, "team_id = ? and check_name = ?", team.ID, check.FetchName())
	if res.Error != nil {
		errorOutGraceful(c, res.Error)
		return
	}

	var results []ResultEntry
	res = db.Order("time desc").Find(&results, "team_id = ? and name = ?", team.ID, check.FetchName())
	if res.Error != nil {
		errorOutGraceful(c, res.Error)
		return
	}
	c.HTML(http.StatusOK, "check.html", pageData(c, team.Name+": "+check.FetchName(), gin.H{"team": team, "check": check, "sla": sla, "results": results}))
}

func viewPCR(c *gin.Context) {
	team := getUser(c)

	var submissions []InjectSubmission
	if team.IsAdmin() {
		// get all pcr entries
		res := db.Order("time desc").Preload("Team").Where("inject_id = 1 and feedback = ''").Find(&submissions)
		if res.Error != nil {
			errorPrint(res.Error)
			return
		}
	} else {
		// get all pcr entries
		res := db.Order("time desc").Preload("Team").Where("inject_id = 1 and feedback = '' and team_id = ?", team.ID).Find(&submissions)
		if res.Error != nil {
			errorPrint(res.Error)
			return
		}
	}

	// Load all content from on disk
	// TODO: skip the middleman and just do this when received?
	for i, submission := range submissions {
		submissions[i].Content = readInject(submission)
	}

	c.HTML(http.StatusOK, "pcr.html", pageData(c, "PCRs", gin.H{"team": team, "creds": ct.Creds, "submissions": submissions}))
}

func viewPersist(c *gin.Context) {
	// previous rounds from db
	var previous []Persist
	db.Preload(clause.Associations).Find(&previous)
	c.HTML(http.StatusOK, "persists.html", pageData(c, "Persists", gin.H{"current": persistHits, "previous": previous}))
}

func scorePersist(c *gin.Context) {
	teamMutex.Lock()
	defer teamMutex.Unlock()

	// Identify box (team and check)
	remoteIPRaw, _ := c.RemoteIP()
	if remoteIPRaw == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "your IP is nil?! (contact the organizer please)"})
		return
	}
	remoteIP := remoteIPRaw.String()

	team, boxName, err := boxFromIP(remoteIP)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "your IP is not a valid box"})
		return
	}

	offender, err := tokenToTeam(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	if team.ID == offender.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you can't hack yourself..."})
		return
	}

	// Initialize map if not already created
	if _, ok := persistHits[team.ID]; !ok {
		persistHits[team.ID] = make(map[string][]uint)
	}

	// Ensure not a duplicate
	for _, persist := range persistHits[team.ID][boxName] {
		if persist == offender.ID {
			c.JSON(http.StatusOK, "OK")
			return
		}
	}

	// Append offender ID
	persistHits[team.ID][boxName] = append(persistHits[team.ID][boxName], offender.ID)
	c.JSON(http.StatusOK, "OK")

}

func viewRed(c *gin.Context) {
	// yeet
}

func submitRed(c *gin.Context) {
	// yeet
}

func viewResets(c *gin.Context) {
	// yeet
}

func submitReset(c *gin.Context) {
	// yeet
}

func viewInjects(c *gin.Context) {
	team := getUser(c)

	// view all injects
	var injects []Inject

	// populate status for each inject
	if !team.IsAdmin() {
		res := db.Find(&injects)
		if res.Error != nil {
			errorOutGraceful(c, res.Error)
			return
		}

		for i, inj := range injects {
			var submissions []InjectSubmission
			res := db.Where("team_id = ? and inject_id = ?", team.ID, inj.ID).Find(&submissions)
			if res.Error != nil {
				errorOutGraceful(c, res.Error)
				return
			}
			if len(submissions) != 0 {
				for _, sub := range submissions {
					if sub.Graded {
						injects[i].Status = GRADED
						break
					}
				}
				if injects[i].Status != GRADED {
					injects[i].Status = SUBMITTED
				}
			}
		}
	} else {
		res := db.Preload("Submissions").Find(&injects)
		if res.Error != nil {
			errorOutGraceful(c, res.Error)
			return
		}
	}

	c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{"injects": injects, "time": time.Now()}))
}

func viewInject(c *gin.Context) {
	// view individual inject
	injectID, err := strconv.Atoi(c.Param("inject"))
	if err != nil {
		errorOutAnnoying(c, errors.New("invalid inject id (not a number)"))
		return
	}

	var inject Inject
	res := db.First(&inject, "id = ?", injectID)
	if res.Error != nil {
		errorOutAnnoying(c, errors.New("invalid inject id"))
		return
	}

	team := getUser(c)
	var submissions []InjectSubmission
	if team.IsAdmin() {
		res := db.Preload("Team").Find(&submissions, "inject_id = ?", inject.ID)
		if res.Error != nil {
			errorOutGraceful(c, err)
			return
		}
	} else {
		res := db.Order("time desc").Find(&submissions, "team_id = ? and inject_id = ?", team.ID, inject.ID)
		if res.Error != nil {
			errorOutGraceful(c, err)
			return
		}
	}

	c.HTML(http.StatusOK, "inject.html", pageData(c, "injects", gin.H{"inject": inject, "submissions": submissions}))
}

func submitInject(c *gin.Context) {
	team := getUser(c)
	c.Request.ParseForm()
	action := c.Request.Form.Get("action")

	injectID, err := strconv.Atoi(c.Param("inject"))
	if err != nil {
		errorOutAnnoying(c, errors.New("invalid inject id (not a number)"))
		return
	}

	var inject Inject
	res := db.Find(&inject, "id = ?", injectID)
	if res.Error != nil {
		errorOutAnnoying(c, errors.New("invalid inject id"))
		return
	}

	if !team.IsAdmin() && action == "" {
		file, err := c.FormFile("submission")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(int(inject.ID)))
			return
		}
		newSubmission := InjectSubmission{
			Time:     time.Now(),
			Updated:  time.Now(),
			TeamID:   team.ID,
			InjectID: uint(inject.ID),
			FileName: file.Filename,
			DiskFile: uuid.New().String(),
		}

		if err := c.SaveUploadedFile(file, "submissions/"+newSubmission.DiskFile); err != nil {
			c.HTML(http.StatusOK, "injects.html", pageData(c, "Injects", gin.H{"error": "unable to save file"}))
			return
		}

		if res := db.Save(&newSubmission); res.Error != nil {
			c.HTML(http.StatusOK, "injects.html", pageData(c, "Injects", gin.H{"error": res.Error}))
			return
		}
	} else if action == "invalid" {
		submissionId, err := strconv.Atoi(c.Request.Form.Get("submission"))
		if err != nil {
			errorOutAnnoying(c, errors.New("submissionId is not a number"))
			return
		}
		var submission InjectSubmission
		res := db.Find(&submission, "id = ? and team = ? and inject_id = ?", submissionId, team.Name, inject.ID)
		if res.Error != nil {
			errorOutGraceful(c, err)
			return
		}
		fmt.Println(submission, err, submissionId)
		if err != nil || submission.Updated.IsZero() {
			errorOutAnnoying(c, errors.New("invalid diskfile passed to inject invalidation"))
			return
		}
		submission.Invalid = true
		submission.Updated = time.Now()
		res = db.Save(submission)
		if res.Error != nil {
			errorPrint(res.Error)
		}

	}
	// mark invalid:
	// mark invalid
	// grade:
	// check if admin
	// check for team/inject/grade
	c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(int(inject.ID)))
}

func gradeInject(c *gin.Context) {
	var injects []Inject
	res := db.Find(&injects)
	if res.Error != nil {
		errorPrint(res.Error)
		return
	}

	injectId, err := strconv.Atoi(c.Param("inject"))
	if err != nil || injectId > len(injects)-1 {
		errorOutAnnoying(c, errors.New("invalid inject id"))
		return
	}

	team := getUser(c)
	var submission InjectSubmission

	if team.IsAdmin() {
		submissionId, err := strconv.Atoi(c.Param("submission"))
		if err != nil {
			errorOutAnnoying(c, errors.New("submissionId is not a number"))
			return
		}
		res := db.Find(&submission, "id = ?, team = ?, inject_id = ?", submissionId, team.Name, injectId)
		if res.Error != nil {
			errorOutGraceful(c, err)
			return
		}
	} else {
		errorOutAnnoying(c, errors.New("non-admin attempted grade access"))
		return
	}

	c.HTML(http.StatusOK, "grade.html", pageData(c, "grading", gin.H{"submission": submission}))

}

func submitInjectGrade(c *gin.Context) {
	team := c.PostForm("team")
	injectId, err := strconv.Atoi(c.PostForm("injectId"))
	if err != nil {
		errorOutGraceful(c, err)
		return
	}
	submissionId, err := strconv.Atoi(c.Param("submission"))
	if err != nil {
		errorOutAnnoying(c, errors.New("submissionId is not a number"))
		return
	}

	var submission InjectSubmission

	res := db.Find(&submission, "id = ?, team = ?, inject_id = ?", submissionId, team, injectId)
	if res.Error != nil {
		errorOutGraceful(c, err)
		return
	}

	submission.Score, err = strconv.Atoi(c.PostForm("grade"))
	if err != nil {
		errorOutGraceful(c, err)
		return
	}
	submission.Feedback = c.PostForm("feedback")

	res = db.Save(&submission)
	if res.Error != nil {
		errorPrint(res.Error)
	}

	fmt.Println("Grade: ", submission.Score, "\nFeedback: ", submission.Feedback)
	c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(injectId))
}

func viewScores(c *gin.Context) {
	if !dwConf.Verbose && !getUser(c).IsAdmin() {
		errorOutAnnoying(c, errors.New("access to score without admin or verbose mode"))
	}

	teamMutex.Lock()
	var records []TeamRecord
	res := db.Limit(len(dwConf.Team)).Where("round = ?", roundNumber-1).Preload("Team").Order("time desc").Find(&records)
	if res.Error != nil {
		errorOutGraceful(c, res.Error)
		teamMutex.Unlock()
		return
	}
	teamMutex.Unlock()

	// Calculate totals.
	for i, rec := range records {
		records[i].Total = calculateScoreTotal(rec)
	}

	// Sort by total score.
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Total > records[j].Total
	})

	graphScores(records)
	team := getUserOptional(c)

	c.HTML(http.StatusOK, "scores.html", pageData(c, "scores", gin.H{"records": records, "team": team}))
}

func exportTeamData(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("team"))
	if err != nil {
		errorOutAnnoying(c, errors.New("invalid team id: "+c.Param("team")))
		return
	}
	team := validateTeam(c, uint(id))
	csvString := "time,round,service,inject,sla,"
	for _, b := range dwConf.Box {
		for _, c := range b.CheckList {
			csvString += c.FetchName() + ","
		}
	}
	csvString += "total\n"

	var records []TeamRecord
	res := db.Find(&records, "team = ?", team.Name)
	if res.Error != nil {
		errorOutGraceful(c, res.Error)
		return
	}

	for _, r := range records {
		csvString += r.Time.In(loc).Format("03:04:05 PM") + ","
		csvString += strconv.Itoa(r.Round) + ","
		csvString += strconv.Itoa(r.ServicePoints) + ","
		csvString += strconv.Itoa(r.InjectPoints) + ","
		statusString := ""
		for _, c := range r.Results {
			if c.Status {
				statusString += "up,"
			} else {
				statusString += "down,"
			}
		}
		csvString += "-" + strconv.Itoa(r.SlaViolations*dwConf.SlaPoints) + ","
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
func validateTeam(c *gin.Context, id uint) TeamData {
	team := getUser(c)
	if team.ID == id {
		return team
	} else if team.IsAdmin() {
		if realTeam, err := dwConf.GetTeam(id); err == nil {
			return realTeam
		} else {
			errorPrint(err)
		}
	}
	errorOutAnnoying(c, errors.New("team could not be validated"))
	return TeamData{}
}

func (m *config) IsValid(team TeamData, id string) bool {
	if team.Name == id || team.IsAdmin() {
		return true
	}
	return false
}
