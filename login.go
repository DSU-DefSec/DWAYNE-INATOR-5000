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
	r.Use(sessions.Sessions("mew", cookie.NewStore([]byte(getUUID()))))
}

// authRequired provides authentication middleware for ensuring that a user is logged in.
func authRequired(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	if user == nil {
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

	// Validate form input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.HTML(http.StatusBadRequest, "login.html", pageData(c, "login", gin.H{"error": "Username or password can't be empty."}))
		return
	}

	err := errors.New("Invalid username or password.")

	for _, t := range mewConf.Admin {
		if username == t.Identifier && password == t.Pw {
			err = nil
		}
	}

	for _, t := range mewConf.Red {
		if username == t.Identifier && password == t.Pw {
			err = nil
		}
	}

	for _, t := range mewConf.Team {
		if username == t.Identifier && password == t.Pw {
			err = nil
		}
	}

	if err != nil {
		c.HTML(http.StatusBadRequest, "login.html", pageData(c, "login", gin.H{"error": err.Error()}))
		return
	}

	// Save the username in the session
	session.Set("user", username)
	if err := session.Save(); err != nil {
		c.HTML(http.StatusBadRequest, "login.html", pageData(c, "login", gin.H{"error": "Failed to save session."}))
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (t teamData) IsAdmin() bool {
	for _, admin := range mewConf.Admin {
		if admin.Identifier == t.Identifier {
			return true
		}
	}
	return false
}

func (t teamData) IsRed() bool {
	for _, admin := range mewConf.Red {
		if admin.Identifier == t.Identifier {
			return true
		}
	}
	return false
}

func getUser(c *gin.Context) teamData {
	if team := getUserOptional(c); team.Identifier == "" {
		errorOutAnnoying(c, errors.New("invalid team"))
	} else {
		return team
	}
	return teamData{}
}

func getUserOptional(c *gin.Context) teamData {
	userName := sessions.Default(c).Get("user")
	if userName != nil {
		for _, team := range mewConf.Admin {
			if team.Identifier == userName {
				return team
			}
		}
		for _, team := range mewConf.Team {
			if team.Identifier == userName {
				return team
			}
		}
		for _, team := range mewConf.Red {
			if team.Identifier == userName {
				return team
			}
		}
	}
	return teamData{}
}

func getFromAllUsers(username string) {
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	if user == nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	session.Delete("user")
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}
