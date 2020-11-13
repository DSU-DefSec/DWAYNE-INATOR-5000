package main

import (
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DSU-DefSec/mew/checks"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

func viewStatus(c *gin.Context) {
	records, err := getStatus()
	if err != nil {
		errorOutGraceful(c, err)
	}
	team := teamData{}
	team, _ = mewConf.GetTeam(getUser(c))
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard", gin.H{"records": records, "team": team.Identifier}))
}

func viewTeam(c *gin.Context) {
	team := getTeamWithAdmin(c)
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
	team := teamData{}
	pcrItems := []checks.PcrData{}
	if mewConf.isAdmin(getUser(c)) {
		// debugPrint("getting all pcr items")
		pcrItems, err = getAllTeamPCRItems()
		// debugPrint("pcrItems is", pcrItems)
		if err != nil {
			errorOutGraceful(c, err)
		}
	} else {
		team, err = mewConf.GetTeam(getUser(c))
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
		userLookup := c.Request.Form.Get("username")
		if userLookup != "" {
			validUser := false
			allUsernames := []string{}
			for _, cred := range mewConf.Creds {
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
			for _, t := range mewConf.Team {
				pwLookup[t.Identifier] = make(map[string]string)
				for _, b := range mewConf.Box {
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
			team, err = mewConf.validateTeamIndex(getUser(c), c.Request.Form.Get("team"))
			if err != nil {
				if err.Error() != "unauthorized team" {
					errorOutAnnoying(c, err)
				}
			}
		}
	} else {
		team, err = mewConf.GetTeam(getUser(c))
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
		debugPrint("submitpcr:", err)
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

	// sort redRecords based on inverse redPoints
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Total > records[j].Total
	})

	// sort redRecords based on inverse redPoints
	redRecords := []teamRecord{}
	for _, rec := range records {
		redRecords = append(redRecords, rec)
	}

	sort.SliceStable(redRecords, func(i, j int) bool {
		return (redRecords[i].RedContrib + redRecords[i].RedDetract) > (redRecords[j].RedContrib + redRecords[j].RedDetract)
	})

	team := teamData{}
	team, _ = mewConf.GetTeam(getUser(c))

	c.HTML(http.StatusOK, "scores.html", pageData(c, "scores", gin.H{"records": records, "redRecords": redRecords, "team": team}))
}

func exportTeamData(c *gin.Context) {
	team := getTeamWithAdmin(c)
	csvString := "time,round,service,inject,sla,"
	for _, b := range mewConf.Box {
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
	team, err := mewConf.GetTeam(getUser(c))
	if err == nil {
		newGinMap["team"] = team
	}
	newGinMap["admin"] = mewConf.isAdmin(getUser(c))
	newGinMap["red"] = mewConf.isRed(getUser(c))
	newGinMap["event"] = mewConf.Event
	newGinMap["m"] = mewConf
	newGinMap["loc"] = loc
	for key, value := range ginMap {
		newGinMap[key] = value
	}
	return newGinMap
}

func persistHandler(c *gin.Context) {
	// if cyberconquest
	if mewConf.Kind != "blue" {
		redTeamId := c.Param("id")
		var sourceIp string
		if ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr)); err == nil {
			sourceIp = ip
		} else {
			c.JSON(400, gin.H{"error": "Invalid source IP"})
			return
		}

		debugPrint("Source ip is ", sourceIp)
		var redTeam, sourceTeam, sourceBox string

		for _, team := range mewConf.Team {
			if team.Red == redTeamId {
				redTeam = mewConf.GetIdentifier(team.Display)
			}
		}

		if redTeam == "" {
			c.JSON(400, gin.H{"error": "Invalid red team token"})
			return
		}

		for _, team := range mewConf.Team {
			if sourceTeam == "" {
				for _, box := range mewConf.Box {
					if team.Prefix+box.Suffix == sourceIp {
						sourceTeam = mewConf.GetIdentifier(team.Display)
						sourceBox = box.Name
						break
					}
				}
			}
		}

		if sourceBox == "" {
			c.JSON(400, gin.H{"error": "Source IP " + sourceIp + " is not a from a valid box"})
			return
		}

		// create map is not already created
		if _, ok := redPersists[sourceTeam]; !ok {
			redPersists[sourceTeam] = make(map[string][]string)
		}
		// if not already persisted for the given team, add team
		if teamList, ok := redPersists[sourceTeam][sourceBox]; !ok {
			redPersists[sourceTeam][sourceBox] = append(redPersists[sourceTeam][sourceBox], redTeam)
		} else {
			found := false
			for _, team := range teamList {
				if team == redTeam {
					found = true
				}
			}
			if !found {
				redPersists[sourceTeam][sourceBox] = append(redPersists[sourceTeam][sourceBox], redTeam)
			}
		}
	}
	c.JSON(200, gin.H{"status": "ok"})
}
