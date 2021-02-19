package checks

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

type Sql struct {
	checkBase
	Kind  string
	Query []queryData
}

type queryData struct {
	UseRegex bool
	Contains bool
	Database string
	Table    string
	Column   string
	Output   string
}

func (c Sql) Run(teamName, boxIp string, res chan Result) {
	username, password := getCreds(c.CredLists, teamName, c.Name)

	// Run query
	q := c.Query[rand.Intn(len(c.Query))]

	db, err := sql.Open(c.Kind, fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, boxIp, c.Port, q.Database))
	if err != nil {
		res <- Result{
			Error: "creating db handle failed",
			Debug: "error: " + err.Error() + ", creds " + username + ":" + password,
		}
		return
	}
	defer db.Close()

	// Check db connection
	err = db.PingContext(context.TODO())
	if err != nil {
		res <- Result{
			Error: "db did not respond to ping",
			Debug: err.Error(),
		}
		return
	}

	// Query the DB
	// TODO: This is SQL injectable. Figure out Paramerterized queries
	rows, err := db.QueryContext(context.TODO(), fmt.Sprintf("SELECT %s FROM %s;", q.Column, q.Table))
	if err != nil {
		res <- Result{
			Error: "could not query db for database " + q.Database + " table " + q.Table + " column " + q.Column,
			Debug: err.Error(),
		}
		return
	}
	defer rows.Close()

	var output string
	if q.Output != "" {
		foundSwitch := false
		// Check the rows
		for rows.Next() {
			// Grab a value
			err := rows.Scan(&output)
			if err != nil {
				res <- Result{
					Error: "could not get row values",
					Debug: err.Error(),
				}
				return
			}
			if q.Contains {
				if q.UseRegex {
					re := regexp.MustCompile(q.Output)
					found := re.Find([]byte(output))
					if len(found) != 0 {
						foundSwitch = true
						break
					}
				} else {
					if strings.Contains(output, q.Output) {
						foundSwitch = true
						break
					}
				}
			} else {
				if q.UseRegex {
					re := regexp.MustCompile(q.Output)
					if re.Match([]byte(output)) {
						foundSwitch = true
						break
					}
				} else {
					if strings.TrimSpace(output) == q.Output {
						foundSwitch = true
						break
					}
				}
			}
		}
		if !foundSwitch {
			res <- Result{
				Error: "query output didn't contain value",
				Debug: "database " + q.Database + " table " + q.Table + " column " + q.Column + " didn't contain " + q.Output,
			}
			return
		}
		// Check for error in the rows
		if rows.Err() != nil {
			res <- Result{
				Error: "sql rows experienced an error",
				Debug: err.Error(),
			}
			return
		}
	}

	res <- Result{
		Status: true,
		Debug:  "creds used were " + username + ":" + password,
	}
}
