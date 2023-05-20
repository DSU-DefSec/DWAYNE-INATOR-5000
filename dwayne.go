package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	defaultDelay  = 60
	defaultJitter = 30
)

var (
	dwConf = &config{}
	db     = &gorm.DB{}

	startTime     time.Time
	delayedChecks struct {
		Box []Box
	}

	configPath = flag.String("c", "dwayne.conf", "configPath")
	debug      = flag.Bool("d", false, "debugFlag")

	roundNumber int
	resetIssued bool
	pauseTime   time.Time
	ct          CredentialTable
	loc         *time.Location
	ZeroTime    time.Time

	teamMutex       = &sync.Mutex{}
	persistMutex    = &sync.Mutex{}
	agentMutex      = &sync.Mutex{}
	adjustmentMutex = &sync.Mutex{}
)

func errorPrint(a ...interface{}) {
	log.Printf("[ERROR] %s", fmt.Sprintln(a...))
}

func debugPrint(a ...interface{}) {
	if *debug {
		log.Printf("[DEBUG] %s", fmt.Sprintln(a...))
	}
}

func init() {
	flag.Parse()
}

func main() {
	readConfig(dwConf)
	err := checkConfig(dwConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	// Load timezone
	loc, err = time.LoadLocation(dwConf.Timezone)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "invalid timezone"))
	}

	// we've evolved to... superjank.
	localTime := time.FixedZone("time-local", func() int { _, o := time.Now().Zone(); return o }())
	dateDiff := time.Date(0, 1, 1, 0, 0, 0, 0, localTime).Sub(time.Date(0, 1, 1, 0, 0, 0, 0, loc))
	ZeroTime = time.Date(0, 1, 1, 0, 0, 0, 0, loc).Add(dateDiff)
	time.Local = loc

	// Open database
	db, err = gorm.Open(sqlite.Open(dwConf.DBPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database!")
	}

	db.AutoMigrate(&ResultEntry{}, &TeamRecord{}, &Inject{}, &InjectSubmission{}, &TeamData{}, &SLA{}, &Persist{})

	// Initialize manual adjustments map
	manualAdjustments = make(map[uint]int)

	if dwConf.Persists {
		persistHits = make(map[uint]map[string][]uint)
	}

	// Assign team IDs sequentially
	for i := range dwConf.Team {
		dwConf.Team[i].ID = uint(i + 1)
	}

	// Fill uptime hits with engine start time
	if dwConf.Uptime {
		initAgentTime := time.Now().In(loc)
		agentHits = make(map[uint]map[string]time.Time)
		for _, t := range dwConf.Team {
			agentHits[t.ID] = make(map[string]time.Time)
			for _, b := range dwConf.Box {
				agentHits[t.ID][b.Name] = initAgentTime
			}
		}
	}

	// Save into DB if not already in there.
	var teams []TeamData
	res := db.Find(&teams)
	if res.Error == nil && len(teams) == 0 {
		for _, team := range dwConf.Team {
			if res := db.Create(&team); res.Error != nil {
				log.Fatal("unable to save team in database")
			}
		}
	}

	// Initialize mutex for credential table
	ct.Mutex = &sync.Mutex{}

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Add... add function
	r.SetFuncMap(template.FuncMap{
		"increment": func(x int) int {
			return x + 1
		},
		"mul": func(x, y int) int {
			return x * y
		},
		"rand": func() string {
			// Lol
			return uuid.New().String()
		},
	})

	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./assets")
	initCookies(r)

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "404.html", pageData(c, "404 Not Found", nil))
	})

	// Routes
	routes := r.Group("/")
	{
		routes.GET("/", viewStatus)
		routes.GET("/info", func(c *gin.Context) {
			c.HTML(http.StatusOK, "info.html", pageData(c, "Information", nil))
		})
		routes.GET("/login", func(c *gin.Context) {
			if getUserOptional(c).IsValid() {
				c.Redirect(http.StatusSeeOther, "/")
			}
			c.HTML(http.StatusOK, "login.html", pageData(c, "Login", nil))
		})
		routes.GET("/forbidden", func(c *gin.Context) {
			c.HTML(http.StatusOK, "forbidden.html", pageData(c, "Forbidden", nil))
		})
		routes.POST("/login", login)
		if dwConf.Persists {
			routes.GET("/persist/:token", scorePersist)
			routes.GET("/persist", viewPersist)
		}

		if dwConf.Uptime {
			routes.GET("/agents", func(c *gin.Context) {
				c.HTML(http.StatusOK, "agents.html", pageData(c, "Forbidden", gin.H{"agentHits": agentHits, "uptimeSLA": uptimeSLA, "now": time.Now()}))
			})
			// aeacus compatability line :shrug:
			routes.GET("/status/:id/:boxName", func(c *gin.Context) {
				c.JSON(http.StatusOK, nil)
			})
			routes.POST("/update", scoreAgent)
		}

		// Has API key check. If more API routes are added in the future,
		// add own endpoint group with auth middleware
		routes.POST("/injects", createInject)
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/logout", logout)

		// Team Information
		authRoutes.GET("/export/:team", exportTeamData)
		authRoutes.GET("/team/:team", viewTeam)
		authRoutes.GET("/team/:team/:check", viewCheck)
		//authRoutes.GET("/uptime/:team", viewUptime)

		// PCRs
		authRoutes.GET("/pcr", viewPCR)
		if dwConf.EasyPCR {
			authRoutes.POST("/pcr", submitPCR)
		}

		/*
			// Red Team
			// not implemented yet
			authRoutes.GET("/red", viewRed)
			authRoutes.POST("/red", submitRed)
		*/

		// Injects
		authRoutes.GET("/injects", viewInjects)
		authRoutes.GET("/injects/feed", injectFeed)
		authRoutes.GET("/injects/view/:inject", viewInject)
		authRoutes.POST("/injects/view/:inject", submitInject)
		authRoutes.GET("/injects/delete/:inject", deleteInject)
		authRoutes.POST("/injects/view/:inject/:submission/invalid", invalidateInject)
		authRoutes.GET("/injects/view/:inject/:submission/grade", gradeInject)
		authRoutes.POST("/injects/view/:inject/:submission/grade", submitInjectGrade)

		// Inject submissions
		authRoutes.Static("/submissions", "./submissions")
		r.Static("/inject_files", "./injects")

		// Settings
		authRoutes.GET("/settings", viewSettings)
		authRoutes.POST("/settings/reset", func(c *gin.Context) {
			team := getUser(c)
			if !team.IsAdmin() {
				errorOutAnnoying(c, errors.New("non-admin tried to issue a scoring engine reset: "+c.Param("team")))
				return
			}

			teamMutex.Lock()
			resetIssued = true

			db.Exec("DELETE FROM result_entries")
			db.Exec("DELETE FROM team_records")
			db.Exec("DELETE FROM inject_submissions")
			db.Exec("DELETE FROM slas")
			db.Exec("DELETE FROM persists")

			// Deal with cache
			cachedStatus = []TeamRecord{}
			cachedRound = 0
			roundNumber = 0
			startTime = time.Now().In(loc)
			persistHits = make(map[uint]map[string][]uint)
			teamMutex.Unlock()

			c.Redirect(http.StatusSeeOther, "/")
		})
		authRoutes.POST("/settings/start", func(c *gin.Context) {
			team := getUser(c)
			if !team.IsAdmin() {
				errorOutAnnoying(c, errors.New("non-admin tried to start scoring: "+c.Param("team")))
				return
			}
			dwConf.Running = true
			c.Redirect(http.StatusSeeOther, "/settings")
		})
		authRoutes.POST("/settings/stop", func(c *gin.Context) {
			team := getUser(c)
			if !team.IsAdmin() {
				errorOutAnnoying(c, errors.New("non-admin tried to start scoring: "+c.Param("team")))
				return
			}
			dwConf.Running = false
			resetIssued = true
			pauseTime = time.Now()
			c.Redirect(http.StatusSeeOther, "/settings")
		})
		authRoutes.POST("/settings/adjust", setManualAdjustment)

		// Resets
		authRoutes.GET("/reset", viewResets)
		authRoutes.POST("/reset/:id", submitReset)
	}

	var injects []Inject
	res = db.Find(&injects)
	if res.Error != nil {
		errorPrint(res.Error)
		return
	}

	if len(injects) == 0 {

		if !dwConf.NoPasswords {
			pwChangeInject := Inject{
				Time:  time.Now(),
				Title: "Password Changes",
				Body:  "Submit your password changes here!",
			}
			res := db.Create(&pwChangeInject)
			if res.Error != nil {
				errorPrint(res.Error)
				return
			}
		}

		var configInjects struct {
			Inject []Inject
		}

		fileContent, err := os.ReadFile("./injects.conf")
		if err != nil {
			log.Println("[WARN] Injects file (injects.conf) not found:", err)
		} else {
			if md, err := toml.Decode(string(fileContent), &configInjects); err != nil {
				log.Fatalln(err)
			} else {
				for _, undecoded := range md.Undecoded() {
					errMsg := "[WARN] Undecoded injects configuration key \"" + undecoded.String() + "\" will not be used."
					configErrors = append(configErrors, errMsg)
					log.Println(errMsg)
				}
			}

			for _, inject := range configInjects.Inject {
				res := db.Create(&inject)
				if res.Error != nil {
					errorPrint(res.Error)
					return
				}
			}
		}

	} else {
		debugPrint("Injects list is not empty, so we are not adding password change inject or processing configured injects")
	}

	// Load delayed checks
	fileContent, err := os.ReadFile("./delayed-checks.conf")
	if err == nil {
		debugPrint("Adding delayed checks...")
		if md, err := toml.Decode(string(fileContent), &delayedChecks); err != nil {
			log.Fatalln(err)
		} else {
			for _, undecoded := range md.Undecoded() {
				errMsg := "[WARN] Undecoded delayed checks configuration key \"" + undecoded.String() + "\" will not be used."
				configErrors = append(configErrors, errMsg)
				log.Println(errMsg)
			}
		}
		for _, b := range delayedChecks.Box {
			if b.Time.IsZero() {
				log.Fatalln("Delayed check box time cannot be zero:", b.Name)
			}
		}

		// sort based on reverse time to inject into checks
		sort.SliceStable(delayedChecks.Box, func(i, j int) bool {
			return delayedChecks.Box[i].InjectTime().After(delayedChecks.Box[i].InjectTime())
		})
	}

	dwConf.Running = true
	go Score(dwConf)
	if dwConf.Https {
		log.Fatal(r.RunTLS(":" + fmt.Sprint(dwConf.Port), dwConf.Cert, dwConf.Key))
	} else {
		log.Fatal(r.Run(":" + fmt.Sprint(dwConf.Port)))
	}
}
