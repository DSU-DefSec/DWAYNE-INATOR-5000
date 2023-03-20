package main

import (
	"log"
	"math/rand"
	"sort"
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

	var record TeamRecord
	res := db.Limit(1).Order("time desc").Find(&record)
	if res.Error == nil {
		roundNumber = record.Round + 1
	} else {
		roundNumber = 1
	}

	// Load earliest startTime from DB record
	record = TeamRecord{}
	res = db.Limit(1).Find(&record)
	if res.Error == nil && !record.Time.IsZero() {
		startTime = record.Time
	} else {
		startTime = time.Now().In(loc)
	}

	rand.Seed(time.Now().UnixNano())
	// checkList = append(checkList, m.Web...)
	//mux := &sync.Mutex{}

	// Build initial PCR table
	if !dwConf.NoPasswords {
		constructPCRState()
	}

	for {

		if m.Running {

			// Check to see if any delayed checks need to be added
			addDelayedChecks()

			log.Println("[SCORE] ===== Round", roundNumber, "(scoring", len(m.Team), "teams)")

			allTeamsWg := &sync.WaitGroup{}
			for _, t := range m.Team {
				allTeamsWg.Add(1)

				go func(team TeamData) {

					wg := &sync.WaitGroup{}
					resChan := make(chan checks.Result)

					//debugPrint("team going into teamrecord is", team)
					newRecord := TeamRecord{
						Time:   time.Now().In(loc),
						TeamID: team.ID,
						Team:   team,
						Round:  roundNumber,
					}

					for _, b := range m.Box {
						for _, check := range b.CheckList {
							wg.Add(1)
							debugPrint("[SCORE] Running check for", team.Name, check)
							go checks.RunCheck(team.ID, team.IP, b.IP, b.Name, check, wg, resChan)
						}
					}

					done := make(chan struct{})
					go func() {
						wg.Wait()
						close(done)
					}()

					// team record
					doneSwitch := false
					for {
						select {
						case res := <-resChan:
							resEntry := ResultEntry{
								Time:   time.Now(),
								TeamID: team.ID,
								Round:  roundNumber,
								Result: checks.Result{
									Name:   res.Name,
									Status: res.Status,
									Error:  res.Error,
									Debug:  res.Debug,
									IP:     res.IP,
									Box:    res.Box,
								},
							}
							newRecord.Results = append(newRecord.Results, resEntry)
						case <-done:
							debugPrint("[SCORE] Checks for team", team.Name, "are done!")
							doneSwitch = true
						}
						if doneSwitch {
							break
						}
					}
					teamMutex.Lock()
					recordsStaging = append(recordsStaging, newRecord)
					teamMutex.Unlock()
					allTeamsWg.Done()
				}(t)
			}
			allTeamsWg.Wait()

			// Process all team records
			teamMutex.Lock()
			if resetIssued {
				debugPrint("[SCORE] Not saving current round, since reset or pause was issued.")
				recordsStaging = []TeamRecord{}
				resetIssued = false
			} else {

				// Assign uptime SLAs if needed
				if dwConf.Uptime {
					agentMutex.Lock()
					for team, boxes := range agentHits {
						for box, lastSeen := range boxes {

							var slaRecord SLA
							result := db.First(&slaRecord, "team_id = ? and reason = ?", team, box)
							if result.Error != nil {
								slaRecord = SLA{
									TeamID: team,
									Reason: box,
								}
							}

							// Issue an SLA only if it's been at least uptimeSLA since the a report AND last SLA
							if time.Since(lastSeen) > uptimeSLA && time.Since(slaRecord.Time) > uptimeSLA {

								slaRecord.Time = time.Now()
								slaRecord.Violations++

								// Inefficient but we'll allow it
								for i, rec := range recordsStaging {
									if rec.TeamID == team {
										recordsStaging[i].SlaViolations++
									}
								}

								if result = db.Save(&slaRecord); result.Error != nil {
									errorPrint(result.Error)
									return
								}
							}
						}
					}
					agentMutex.Unlock()
				}

				adjustmentMutex.Lock()
				for _, rec := range recordsStaging {
					processNewRecord(&rec)
				}
				recordsStaging = []TeamRecord{}
				manualAdjustments = make(map[uint]int)
				adjustmentMutex.Unlock()

				// Calculate persist points
				if dwConf.Persists {
					persistMutex.Lock()
					calculatePersists()
					// Reset persists
					persistHits = make(map[uint]map[string][]uint)
					persistMutex.Unlock()
				}

				// Next round!
				roundNumber++

				if !dwConf.NoPasswords {
					// Build PCR state before sleep.
					// We want submitted PCRs to miss at least one check round.
					debugPrint("[PCR] Constructing PCR state...")
					constructPCRState()
				}
			}
			teamMutex.Unlock()

		}

		jitter := time.Duration(0)
		if dwConf.Jitter != 0 {
			jitter = time.Duration(time.Duration(rand.Intn(dwConf.Jitter+1)) * time.Second)
		}

		log.Println("[SCORE] Sleeping for", dwConf.Delay, "with jitter", jitter)
		time.Sleep((time.Duration(dwConf.Delay) * time.Second) + jitter)

		// If reset was issued during sleep, we ignore it
		if resetIssued == true {
			resetIssued = false
		}
	}
}

func processNewRecord(rec *TeamRecord) {
	var currentRec TeamRecord

	result := db.Limit(1).Preload("Results").Order("time desc").Find(&currentRec, "team_id = ?", rec.Team.ID)
	if result.Error != nil {
		errorPrint(result.Error)
		return
	}

	// Calculate service and SLA values
	for i, res := range rec.Results {
		var slaRecord SLA
		result = db.First(&slaRecord, "team_id = ? and reason = ?", rec.TeamID, res.Name)
		if result.Error != nil {
			slaRecord = SLA{
				TeamID: rec.TeamID,
				Reason: res.Name,
			}
		}
		// O(n^2) lol
		var oldRes ResultEntry
		for _, prevRes := range currentRec.Results {
			if prevRes.Name == res.Name {
				oldRes = prevRes
				break
			}
		}
		rec.Results[i].Points = oldRes.Points
		rec.Results[i].RoundCount = oldRes.RoundCount + 1
		if !res.Status {
			slaRecord.Counter++
			if slaRecord.Counter >= dwConf.SlaThreshold {
				rec.SlaViolations++
				slaRecord.Time = time.Now()
				slaRecord.Violations++
				slaRecord.Counter = 0
			}
		} else {
			slaRecord.Counter = 0
			rec.ServicePoints++
			rec.Results[i].Points++
		}

		if result = db.Save(&slaRecord); result.Error != nil {
			errorPrint(result.Error)
			return
		}
	}

	// Check for manual adjustments
	rec.ManualAdjustment += currentRec.ManualAdjustment
	if adjustment, ok := manualAdjustments[rec.TeamID]; ok {
		rec.ManualAdjustment += adjustment
	}

	// Add other carry-over points
	rec.RedTeamPoints = currentRec.RedTeamPoints
	rec.SlaViolations += currentRec.SlaViolations
	rec.ServicePoints += currentRec.ServicePoints

	// Calculate inject points
	rec.InjectPoints = calculateInjects(currentRec)

	if dwConf.Persists {
		rec.PointsLost += currentRec.PointsLost
		rec.PointsStolen += currentRec.PointsStolen
		rec.PersistPoints += currentRec.PersistPoints
	}

	db.Create(&rec)
}

func calculateInjects(rec TeamRecord) int {
	var injects []Inject

	result := db.Find(&injects)
	if result.Error != nil {
		errorPrint(result.Error)
		return 0
	}

	totalInjectPoints := 0

	// For each inject, get best submission, multiply by points, and add them up
	var bestSubmission InjectSubmission
	for _, inj := range injects {
		res := db.Limit(1).Where("team_id = ?", rec.TeamID).Order("score desc").Find(&bestSubmission)
		if res.Error != nil {
			errorPrint(res.Error)
			return 0
		}
		totalInjectPoints += int((bestSubmission.Score * inj.Points) / 100)
	}

	return totalInjectPoints
}

func calculatePersists() {
	var records []TeamRecord
	res := db.Limit(len(dwConf.Team)).Preload("Team").Preload("Results").Order("time desc").Find(&records)
	if res.Error != nil {
		errorPrint(res.Error)
		return
	}

	// Sort by team ID.
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].TeamID < records[j].TeamID
	})

	// I giveth points, I taketh points
	for team, boxes := range persistHits {
		for box, persists := range boxes {
			if len(persists) > 0 {
				// Get box points to split up.
				victim := records[team-1]
				totalPoints := 0
				for _, res := range victim.Results {
					if box == res.Box {
						if res.Status {
							totalPoints += dwConf.ServicePoints
						}
					}
				}
				distributedPoints := oneOfN(totalPoints, len(persists)+1)
				for _, p := range persists {
					victim.Persists = append(victim.Persists, Persist{
						Round:      roundNumber,
						Box:        box,
						TeamID:     team,
						OffenderID: p,
					})
					records[p-1].PersistPoints += distributedPoints / 2
					records[p-1].PointsStolen += distributedPoints
				}
				victim.PointsLost += distributedPoints * len(persists)
				records[team-1] = victim
			}
		}
	}

	for _, rec := range records {
		rec.Results = []ResultEntry{}
		db.Save(&rec)
	}
}

/*
func percentChangedCreds() map[string]float {
	// get all usernames
	// for each team, see which % of creds exist in pcritems
}
*/
