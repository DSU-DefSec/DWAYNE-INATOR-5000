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
	mewConf = &config{}
	injects = &[]injectData{}

	// Hardcoded CST timezone
	loc, _ = time.LoadLocation("America/Rainy_River")
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
	readConfig(mewConf)
	err := checkConfig(mewConf)
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
		routes.GET("/persist/:id", persistHandler)
		routes.POST("/login", login)
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/logout", logout)
		authRoutes.GET("/export/:team", exportTeamData)
		authRoutes.GET("/pcr", viewPCR)
		authRoutes.POST("/pcr", submitPCR)
		authRoutes.GET("/flags", viewFlags)
		authRoutes.POST("/flags", submitFlags)
		authRoutes.GET("/injects", viewInjects)
		authRoutes.POST("/injects", submitInject)
		authRoutes.GET("/team/:team", viewTeam)
		authRoutes.GET("/team/:team/:check", viewCheck)
	}

	fmt.Println("Refreshing status data from records...")
	initStatus()
	initCreds()

	go Score(mewConf)
	r.Run(":80")
}
