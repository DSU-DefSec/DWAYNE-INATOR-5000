package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	defaultDelay  = 60
	defaultJitter = 30
)

var (
	verbose = false
	dwConf  = &config{}
	injects = []injectData{}

	// Hardcoded CST timezone
	loc, _    = time.LoadLocation("America/Rainy_River")
	locString = "CT"
)

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose/debug output")
	flag.Parse()
	log.SetFlags(0)
}

func errorPrint(a ...interface{}) {
	if verbose {
		log.Printf("[ERROR] %s", fmt.Sprintln(a...))
	}
}

func debugPrint(a ...interface{}) {
	if verbose {
		log.Printf("[DEBUG] %s", fmt.Sprintln(a...))
	}
}

func main() {
	readConfig(dwConf)
	err := checkConfig(dwConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	// Initialize Gin router
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Add... add function
	r.SetFuncMap(template.FuncMap{
		"increment": func(x int) int {
			return x + 1
		},
	})

	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./assets")
	r.Static("/submissions", "./submissions")
	initCookies(r)

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "404.html", nil)
	})

	// Routes
	routes := r.Group("/")
	{
		routes.GET("/", viewStatus)
		routes.GET("/scores", viewScores)
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
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/logout", logout)
		authRoutes.GET("/export/:team", exportTeamData)
		authRoutes.GET("/pcr", viewPCR)
		authRoutes.POST("/pcr", submitPCR)
		authRoutes.GET("/red", viewRed)
		authRoutes.POST("/red", submitRed)
		authRoutes.GET("/injects", viewInjects)
		authRoutes.GET("/injects/:inject", viewInject)
		authRoutes.POST("/injects/:inject", submitInject)
		authRoutes.GET("/injects/:inject/:submission/grade", gradeInject)
		authRoutes.POST("/injects/:inject/:submission/:team/grade", submitInjectGrade)
		authRoutes.GET("/team/:team", viewTeam)
		authRoutes.GET("/team/:team/:check", viewCheck)
	}

	fmt.Println("Refreshing status data from records...")
	initStatus()
	initCreds()
	addInject(injectData{time.Now(), time.Time{}, time.Time{}, "Password Changes", "Submit your password changes here! Please see the team document for more details.", []string{}, 0, 0})
	if err := initInjects(); err != nil {
		errorPrint("couldn't initialize injects:", err)
	}
	if len(injects) == 0 {
		err := addInject(injectData{time.Now(), time.Time{}, time.Time{}, "Password Changes", "Submit your password changes here! Please see the team document for more details.", []string{}, 0, 0})
		if err != nil {
			errorPrint("error adding password change inject:", err)
		}
	}

	go Score(dwConf)
	r.Run(":80")
}
