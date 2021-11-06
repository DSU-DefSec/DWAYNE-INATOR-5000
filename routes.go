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
		res := db.Order("time desc").Preload("Team").Where("inject_id = 1 and graded = true and feedback = ''").Find(&submissions)
		if res.Error != nil {
			errorPrint(res.Error)
			return
		}
	} else {
		// get all pcr entries
		res := db.Order("time desc").Preload("Team").Where("inject_id = 1 and feedback = '' and graded = true and team_id = ?", team.ID).Find(&submissions)
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
			res := db.Order("time desc").Where("team_id = ? and inject_id = ?", team.ID, inj.ID).Find(&submissions)
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

func injectFeed(c *gin.Context) {
	var submissions []InjectSubmission
	team := getUser(c)
	if !team.IsAdmin() {
		errorOutAnnoying(c, errors.New("non-admin feed access"))
		return
	}
	res := db.Find(&submissions, "invalid = false and feedback = '' and score = 0")
	if res.Error != nil {
		errorOutGraceful(c, res.Error)
		return
	}
	c.HTML(http.StatusOK, "injectfeed.html", pageData(c, "inject feed", gin.H{"submissions": submissions}))
}

func createInject(c *gin.Context) {
	apiKey := c.GetHeader("X-Api-Key")
	if apiKey != dwConf.InjectAPIKey {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key"})
		return
	}
	var newInject Inject
	err := c.BindJSON(&newInject)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newInject.Time = time.Now()

	res := db.Create(&newInject)
	if res.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": res.Error})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
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

	if !team.IsAdmin() {
		file, err := c.FormFile("submission")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(int(inject.ID)))
			return
		}
		if injectID != 1 {
			if len(file.Filename) < 4 || file.Filename[len(file.Filename)-4:] != ".pdf" {
				c.HTML(http.StatusOK, "inject.html", pageData(c, "Injects", gin.H{"error": "Your inject upload must have a .PDF extension.", "inject": inject}))
				return
			}
			if len(file.Header["Content-Type"]) != 1 || file.Header["Content-Type"][0] != "application/pdf" {
				c.HTML(http.StatusOK, "inject.html", pageData(c, "Injects", gin.H{"error": "Your inject upload must be a PDF.", "inject": inject}))
				return
			}
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
			c.HTML(http.StatusOK, "inject.html", pageData(c, "Injects", gin.H{"error": "unable to save file", "inject": inject}))
			return
		}

		if res := db.Save(&newSubmission); res.Error != nil {
			c.HTML(http.StatusOK, "inject.html", pageData(c, "Injects", gin.H{"error": res.Error, "inject": inject}))
			return
		}
	} else {
		c.HTML(http.StatusOK, "inject.html", pageData(c, "Injects", gin.H{"error": "Sorry boss, admins can't submit injects.", "inject": inject}))
	}

	c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(int(inject.ID)))
}

func invalidateInject(c *gin.Context) {
	team := getUser(c)
	injectID, err := strconv.Atoi(c.Param("inject"))
	if err != nil {
		errorOutAnnoying(c, errors.New("invalid InjectID"))
		return
	}
	submissionId, err := strconv.Atoi(c.Param("submission"))
	if err != nil {
		errorOutAnnoying(c, errors.New("submissionId is not a number"))
		return
	}
	var submission InjectSubmission
	res := db.Find(&submission, "id = ? and inject_id = ?", submissionId, injectID)
	if res.Error != nil {
		errorOutGraceful(c, err)
		return
	}
	fmt.Println(submission, err, submissionId)
	if err != nil || submission.Updated.IsZero() {
		errorOutAnnoying(c, errors.New("invalid team or inject id"))
		return
	}
	if !team.IsAdmin() && team.ID != submission.TeamID {
		errorOutAnnoying(c, errors.New("non-admin and non-team invalidation access"))
		return
	}
	submission.Invalid = true
	submission.Updated = time.Now()
	res = db.Save(submission)
	if res.Error != nil {
		errorPrint(res.Error)
	}
	c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(int(submission.InjectID)))
}

func gradeInject(c *gin.Context) {
	var injects []Inject
	res := db.Find(&injects)
	if res.Error != nil {
		errorPrint(res.Error)
		return
	}

	injectId, err := strconv.Atoi(c.Param("inject"))
	if err != nil {
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
		res := db.First(&submission, "id = ? and inject_id = ?", submissionId, injectId)
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
	submissionId, err := strconv.Atoi(c.Param("submission"))
	if err != nil {
		errorOutAnnoying(c, errors.New("submissionId is not a number"))
		return
	}

	var submission InjectSubmission

	res := db.Find(&submission, "id = ?", submissionId)
	if res.Error != nil {
		errorOutGraceful(c, err)
		return
	}

	submission.Score, err = strconv.Atoi(c.PostForm("score"))
	if err != nil {
		errorOutGraceful(c, err)
		return
	}

	submission.Graded = true
	submission.Feedback = c.PostForm("feedback")
	res = db.Save(&submission)
	if res.Error != nil {
		errorPrint(res.Error)
	}

	fmt.Println("Score: ", submission.Score, "\nFeedback: ", submission.Feedback)
	c.Redirect(http.StatusSeeOther, "/injects/"+strconv.Itoa(int(submission.InjectID)))
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
