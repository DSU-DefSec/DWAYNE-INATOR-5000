package main

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
	"github.com/pkg/errors"
)

func Score(m *config) {
	err := checkConfig(dwConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	initRoundNumber(m)
	rand.Seed(time.Now().UnixNano())
	// checkList = append(checkList, m.Web...)
	mux := &sync.Mutex{}
	for {
		debugPrint("===================================")
		debugPrint("[SCORE] round", roundNumber)
		allTeamsWg := &sync.WaitGroup{}
		for _, t := range m.Team {
			allTeamsWg.Add(1)
			go func(team teamData) {
				wg := &sync.WaitGroup{}
				resChan := make(chan checks.Result)

				newRecord := teamRecord{
					Time:  time.Now().In(loc),
					Team:  team.Identifier,
					Round: roundNumber,
				}

				for _, b := range m.Box {
					for _, check := range b.CheckList {
						wg.Add(1)
						go checks.RunCheck(team.Identifier, team.Ip, b.Ip, b.Name, check, wg, resChan)
					}
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
							Team:  team.Identifier,
							Round: roundNumber,
							Result: checks.Result{
								Name:   res.Name,
								Status: res.Status,
								Error:  res.Error,
								Debug:  res.Debug,
								Ip:     res.Ip,
								Box:    res.Box,
							},
						}
						newRecord.Checks = append(newRecord.Checks, resEntry)
					case <-done:
						debugPrint("[SCORE] checks for team", team.Identifier, "are done")
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
		if dwConf.Jitter != 0 {
			jitter = time.Duration(time.Duration(rand.Intn(dwConf.Jitter+1)) * time.Second)
		}
		debugPrint("[SCORE] sleeping for", dwConf.Delay, "with jitter", jitter)
		time.Sleep((time.Duration(dwConf.Delay) * time.Second) + jitter)
	}
}
