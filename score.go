package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/DSU-DefSec/mew/checks"
)

func Score(m *config) {
	fillCheckList(mewConf)
	err := checkConfig(mewConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	initRoundNumber(m)
	rand.Seed(time.Now().UnixNano())
	// checkList = append(checkList, m.Web...)
	mux := &sync.Mutex{}
	for {
		fmt.Println("===================================")
		fmt.Println("[SCORE] round", roundNumber)
		allTeamsWg := &sync.WaitGroup{}
		for _, t := range m.Team {
			allTeamsWg.Add(1)
			go func(team teamData) {
				wg := &sync.WaitGroup{}
				resChan := make(chan checks.Result)

				newRecord := teamRecord{
					Time:  time.Now().In(loc),
					Team:  team,
					Round: roundNumber,
				}

				for _, check := range m.CheckList {
					wg.Add(1)
					go checks.RunCheck(team.Prefix, check, wg, resChan)
				}
				done := make(chan struct{})
				go func() {
					wg.Wait()
					close(done)
				}()
				// team recrd
				doneSwitch := false
				for {
					select {
					case res := <-resChan:
						resEntry := resultEntry{
							Time:  time.Now(),
							Team:  team,
							Round: roundNumber,
							Result: checks.Result{
								Name:   res.Name,
								Status: res.Status,
								Error:  res.Error,
								Debug:  res.Debug,
							},
						}
						newRecord.Checks = append(newRecord.Checks, resEntry)
					case <-done:
						fmt.Println("[SCORE] checks for team", team.Name, "are done")
						doneSwitch = true
					}
					if doneSwitch {
						break
					}
				}
				processTeamRecord(newRecord, mux)
				allTeamsWg.Done()
			}(t)
		}
		allTeamsWg.Wait()
		pushTeamRecords(mux)
		roundNumber++
		jitter := time.Duration(0)
		if mewConf.Jitter != 0 {
			jitter = time.Duration(rand.Intn(mewConf.Jitter + 1))
		}
		fmt.Printf("[SCORE] sleeping for %d with jitter %d\n", mewConf.Delay, jitter)
		time.Sleep((time.Duration(mewConf.Delay) + jitter) * time.Second)
	}
}
