package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/DSU-DefSec/mew/checks"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	defaultDelay  = 60
	defaultJitter = 30
)

var (
	mewConf = &config{}
	injects = &[]injectData{}

	// Hardcoded CST timezone
	loc, _ = time.LoadLocation("America/Rainy_River")
)

func main() {
	readConfig(mewConf)
	err := checkConfig(mewConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	// Initialize Gin router
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./assets")
	initCookies(r)

	// Routes
	routes := r.Group("/")
	{
		routes.GET("/", viewStatus)
		routes.GET("/scores", viewScores)
		routes.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", pageData(c, "login", nil))
		})
		routes.GET("/forbidden", func(c *gin.Context) {
			c.HTML(http.StatusOK, "forbidden.html", pageData(c, "forbidden", nil))
		})
		routes.POST("/login", login)
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/logout", logout)
		authRoutes.GET("/export/:team", exportTeamData)
		authRoutes.GET("/pcr", viewPCR)
		authRoutes.POST("/pcr", submitPCR)
		authRoutes.GET("/injects", viewInjects)
		authRoutes.POST("/injects", submitInject)
		authRoutes.GET("/team/:team", viewTeam)
		authRoutes.GET("/team/:team/:check", viewCheck)
	}

	fmt.Println("Refreshing status data from records...")
	initStatus()

	go Score(mewConf)
	r.Run()
}

func viewStatus(c *gin.Context) {
	records, err := getStatus()
	if err != nil {
		panic(err)
	}
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard", gin.H{"records": records}))
}

func viewTeam(c *gin.Context) {
	team := getTeamWithAdmin(c)
	limit := 10
	history, err := getTeamRecords(team, limit)
	if err != nil {
		errorOutGraceful(c, err)
	}
	record := teamRecord{}
	if len(history) >= 1 {
		record = history[0]
	}
	c.HTML(http.StatusOK, "team.html", pageData(c, "Scoreboard", gin.H{"team": team, "record": record, "history": history, "limit": limit}))
}

func getTeamWithAdmin(c *gin.Context) teamData {
	team, err := mewConf.validateTeamIndex(getUser(c), c.Param("team"))
	if err != nil {
		if err.Error() == "unauthorized team" {
			if !mewConf.isAdmin(getUser(c)) {
				errorOutAnnoying(c, err)
			}
		} else {
			errorOutAnnoying(c, err)
		}
	}
	return team
}

func viewCheck(c *gin.Context) {
	team := getTeamWithAdmin(c)
	check, err := mewConf.getCheck(c.Param("check"))
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
		fmt.Println("viewpcr:", err)
		errorOutGraceful(c, err)
	}
	c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems}))
}

func getPCRWeb(c *gin.Context) ([]pcrData, error) {
	var err error
	team := teamData{}
	pcrItems := []pcrData{}
	if mewConf.isAdmin(getUser(c)) {
		pcrItems, err = getAllTeamPCRItems()
		if err != nil {
			errorOutGraceful(c, err)
		}
	} else {
		team, err = mewConf.getTeam(getUser(c))
		if err != nil {
			errorOutAnnoying(c, err)
		}
		pcrItems, err = getPCRItems(team, checks.Web{})
	}
	return pcrItems, err
}

func submitPCR(c *gin.Context) {
	c.Request.ParseForm()
	team := teamData{}
	var err error
	if mewConf.isAdmin(getUser(c)) {
		team, err = mewConf.validateTeamIndex(getUser(c), c.Request.Form.Get("team"))
		if err != nil {
			if err.Error() != "unauthorized team" {
				errorOutAnnoying(c, err)
			}
		}
	} else {
		team, err = mewConf.getTeam(getUser(c))
		if err != nil {
			errorOutAnnoying(c, err)
		}
	}
	submiterr := parsePCR(team, c.Request.Form.Get("check"), c.Request.Form.Get("pcr"))
	var message string
	if submiterr == nil {
		message = "PCR submitted successfully!"
	}

	pcrItems, err := getPCRWeb(c)
	if err != nil {
		fmt.Println("submitpcr:", err)
		errorOutGraceful(c, err)
	}

	c.HTML(http.StatusOK, "pcr.html", pageData(c, "pcr", gin.H{"pcrs": pcrItems, "error": submiterr, "message": message}))
}

func viewInjects(c *gin.Context) {
	// view all injects and their statuses
	// global injects table
	yeetsauce := []injectData{{time.Now(), "nevah", "inject yeeeet", "bradkjadaD", []string{"file1.txt"}, false, false, "yee"}}
	c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{"injects": yeetsauce}))
}

func submitInject(c *gin.Context) {
	// create submission (team0injects)
	c.HTML(http.StatusOK, "injects.html", pageData(c, "injects", gin.H{}))
}

func viewScores(c *gin.Context) {
	if !mewConf.Verbose && !mewConf.isAdmin(getUser(c)) {
		errorOutAnnoying(c, errors.New("access to score without admin or verbose mode"))
	}
	records, err := getStatus()
	if err != nil {
		errorOutGraceful(c, err)
	}
	if !mewConf.Tightlipped {
		graphScores(records)
	}
	c.HTML(http.StatusOK, "scores.html", pageData(c, "scores", gin.H{"records": records}))
}

func exportTeamData(c *gin.Context) {
	team := getTeamWithAdmin(c)
	csvString := "time,round,service,inject,sla,"
	for _, c := range mewConf.CheckList {
		csvString += c.FetchName() + ","
	}
	csvString += "total\n"
	records, err := getTeamRecords(team, 0)
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
		csvString += "-" + strconv.Itoa(slaViolations*mewConf.SlaPoints) + ","
		csvString += statusString
		csvString += strconv.Itoa(r.Total) + "\n"
	}
	c.Data(200, "text/csv", []byte(csvString))
}

func pageData(c *gin.Context, title string, ginMap gin.H) gin.H {
	newGinMap := gin.H{}
	newGinMap["title"] = title
	newGinMap["user"] = getUser(c)
	team, err := mewConf.getTeam(getUser(c))
	if err == nil {
		newGinMap["team"] = team
	}
	newGinMap["admin"] = mewConf.isAdmin(getUser(c))
	newGinMap["event"] = mewConf.Event
	newGinMap["m"] = mewConf
	newGinMap["loc"] = loc
	for key, value := range ginMap {
		newGinMap[key] = value
	}
	return newGinMap
}
