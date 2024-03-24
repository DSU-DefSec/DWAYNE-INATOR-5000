package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getUUID returns a randomly generated UUID
func getUUID() string {
	return uuid.New().String()
}

// initCookies use gin-contrib/sessions{/cookie} to initalize a cookie store.
// It generates a random secret for the cookie store -- not ideal for continuity or invalidating previous cookies, but it's secure and it works
func initCookies(r *gin.Engine) {
	r.Use(sessions.Sessions("dwayne-inator-5000", cookie.NewStore([]byte(getUUID()))))
}

// authRequired provides authentication middleware for ensuring that a user is logged in.
func authRequired(c *gin.Context) {
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		c.Redirect(http.StatusSeeOther, "/login")
		c.Abort()
	}
	c.Next()
}

// login is a handler that parses a form and checks for specific data
func login(c *gin.Context) {
	session := sessions.Default(c)
	username := c.PostForm("username")
	password := c.PostForm("password")
	var team TeamData

	// Validate form input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.HTML(http.StatusBadRequest, "login.html", pageData(c, "login", gin.H{"error": "Username or password can't be empty."}))
		return
	}

	err := errors.New("Invalid username or password.")

	for _, t := range dwConf.Admin {
		if username == t.Name && password == t.Pw {
			team = t
			err = nil
		}
	}

	for _, t := range dwConf.Red {
		if username == t.Name && password == t.Pw {
			team = t
			err = nil
		}
	}

	for _, t := range dwConf.Team {
		if username == t.Name && password == t.Pw {
			team = t
			err = nil
		}
	}

	if err != nil {
		c.HTML(http.StatusBadRequest, "login.html", pageData(c, "login", gin.H{"error": err.Error()}))
		return
	}

	// Save the username in the session
	session.Set("id", team.ID)
	if err := session.Save(); err != nil {
		c.HTML(http.StatusBadRequest, "login.html", pageData(c, "login", gin.H{"error": "Failed to save session."}))
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (t TeamData) IsAdmin() bool {
	for _, admin := range dwConf.Admin {
		if admin.Name == t.Name {
			return true
		}
	}
	return false
}

func (t TeamData) IsRed() bool {
	for _, admin := range dwConf.Red {
		if admin.Name == t.Name {
			return true
		}
	}
	return false
}

func getUser(c *gin.Context) TeamData {
	if team := getUserOptional(c); team.Name == "" {
		errorOutAnnoying(c, errors.New("invalid team"))
	} else {
		return team
	}
	return TeamData{}
}

func getUserOptional(c *gin.Context) TeamData {
	userID := sessions.Default(c).Get("id")
	if userID != nil {
		return getTeam(userID.(uint))
	}
	return TeamData{}
}

func getTeam(id uint) TeamData {
	for _, team := range dwConf.Admin {
		if team.ID == id {
			return team
		}
	}
	for _, team := range dwConf.Team {
		if team.ID == id {
			return team
		}
	}
	for _, team := range dwConf.Red {
		if team.ID == id {
			return team
		}
	}
	return TeamData{}
}

func getFromAllUsers(username string) {
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	session.Delete("id")
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}
