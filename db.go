package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"sync"

	"github.com/DSU-DefSec/mew/checks"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbName        = "mew"
	dbURI         = "mongodb://localhost:27017"
	mongoClient   *mongo.Client
	mongoCtx      context.Context
	timeConn      time.Time
	roundNumber   int
	prevRecords   = make(map[teamData]teamRecord) // cached team records from last round
	recordStaging = make(map[teamData]teamRecord) // currently being built team records
	redPersists = make(map[string]map[string][]string) // for each team's box, which teams have claimed persistence on it
)
/*
mew
	status
		status is current state of each team
			removed/updated each tiem team<index>-records is yeeted
	injects
		inject data (edited through web interface or config)
	team<index>-results
		each individual result for each check for the tema
	team<index>-records
		team records. contains state of team each check ([]resultnetry) and points (service, inject)
	team<index>-pcr
		password change requests
			one record per service
				if row not found, use default creds
*/

type resultEntry struct {
	Time          time.Time `json:"time,omitempty"`
	Team          teamData  `json:"team,omitempty"`
	Round         int       `json:"round,omitempty"`
	SlaCounter    int       `json:"slacounter,omitempty"`
	SlaViolations int       `json:"slacounter,omitempty"`
	checks.Result
}

type resultWrapper struct {
	Team   teamData
	Result checks.Result
}

type teamRecord struct {
	Time          time.Time     `json:"time,omitempty"`
	Team          teamData      `json:"team,omitempty"`
	Round         int           `json:"round,omitempty"`
	Checks        []resultEntry `json:"checks,omitempty"`
	RedDetract     int       `json:"reddetract,omitempty"`
	RedContrib     int       `json:"redcontrib,omitempty"`
	ServicePoints int           `json:"servicepoints,omitempty"`
	InjectPoints  int           `json:"injectpoints,omitempty"`
	SlaViolations int           `json:"slaviolations,omitempty"`
	Total         int           `json:"total,omitempty"`
}

type adminData struct {
	Name, Pw string
}

type teamData struct {
	Name, Prefix, Pw string
}

type injectSubmission struct {
	Time   time.Time
	Text   string
	Files  []string
	Status string
	Points int
}

type injectData struct {
	Time       time.Time
	Due        string
	Title      string
	Body       string
	Files      []string
	FileUpload bool
	TextEntry  bool
	Status     string
}

type pcrData struct {
	Time  time.Time
	Team  teamData
	Check string
	Creds map[string]string
}

func initDatabase() {
	refresh := false

	if timeConn.IsZero() {
		refresh = true
	} else {
		err := mongoClient.Ping(context.TODO(), nil)
		if err != nil {
			refresh = true
			mongoClient.Disconnect(mongoCtx)
		}
	}
	timeConn = time.Now()

	if refresh {
		fmt.Println("Refreshing mongodb connection...")
		client, err := mongo.NewClient(options.Client().ApplyURI(dbURI))
		if err != nil {
			log.Fatal(err)
		} else {
			mongoClient = client
		}
		ctx := context.TODO()
		err = client.Connect(ctx)
		if err != nil {
			log.Fatal(err)
		} else {
			mongoCtx = ctx
		}
	}
}

func getCollection(collectionName string) *mongo.Collection {
	initDatabase()
	return mongoClient.Database(dbName).Collection(collectionName)
}

func getCheckResults(team teamData, check checks.Check, limit int) ([]resultEntry, error) {
	results := []resultEntry{}
	coll := getCollection(mewConf.GetIdentifier(team.Name) + "results")

	// Create option and index (allow faster & larger sorts)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"time", -1}})
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}

	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return results, err
	}

	cursor, err := coll.Find(context.TODO(), bson.D{{"result.name", check.FetchName()}}, findOptions)
	if err != nil {
		return results, err
	}

	if err := cursor.All(mongoCtx, &results); err != nil {
		return results, err
	}

	return results, nil
}

func getAllTeamPCRItems() ([]pcrData, error) {
	// go through each team and get dem records
	// sort by time
	return []pcrData{}, nil
}

func getPCRItems(team teamData, check checks.Check) ([]pcrData, error) {
	pcrItems := []pcrData{}
	coll := getCollection(mewConf.GetIdentifier(team.Name) + "pcr")

	// Create option and index (allow faster & larger sorts)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"time", -1}})

	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}

	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return pcrItems, err
	}

	var cursor *mongo.Cursor
	if check.FetchName() != "" {
		cursor, err = coll.Find(context.TODO(), bson.D{{"check.name", check.FetchName()}}, findOptions)
	} else {
		cursor, err = coll.Find(context.TODO(), bson.D{}, findOptions)
	}

	if err != nil {
		return pcrItems, err
	}

	if err := cursor.All(mongoCtx, &pcrItems); err != nil {
		return pcrItems, err
	}

	fmt.Println("pcritems is", pcrItems)

	return pcrItems, nil
}

func getTeamRecords(team teamData, limit int) ([]teamRecord, error) {
	records := []teamRecord{}
	coll := getCollection(mewConf.GetIdentifier(team.Name) + "records")

	// Create option and index (allow faster & larger sorts)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"time", -1}})
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}

	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return records, err
	}

	cursor, err := coll.Find(context.TODO(), bson.D{{"team", team}}, findOptions)
	if err != nil {
		return records, err
	}

	if err := cursor.All(mongoCtx, &records); err != nil {
		return records, err
	}

	// Sort -- could make other function or sort in Mongo
	for i := range records {
		records[i].Checks = sortResults(records[i].Checks)
	}

	return records, nil
}

func initStatus() error {
	coll := getCollection("status")
	err := coll.Drop(mongoCtx)
	if err != nil {
		return errors.New("error dropping collection status")
	}
	topBoard, err := groupTeamRecords(mewConf)
	if err != nil {
		return errors.New("error fetching grouped team records")
	}
	if len(topBoard) > 0 {
		topBoardInterface := []interface{}{}
		for _, item := range topBoard {
			topBoardInterface = append(topBoardInterface, item)
		}
		_, err = coll.InsertMany(context.TODO(), topBoardInterface, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func initRoundNumber(m *config) {
	res := resultEntry{}
	coll := getCollection(m.GetIdentifier(m.Team[0].Name) + "results")
	findOptions := options.FindOne()
	findOptions.SetSort(bson.D{{"time", -1}})
	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}
	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		fmt.Println("initRoundNumber:", err)
		roundNumber = 0
		return
	}
	err = coll.FindOne(context.TODO(), bson.D{}, findOptions).Decode(&res)
	if err != nil {
		fmt.Println("initRoundNumber:", err)
		roundNumber = 0
		return
	}
	roundNumber = res.Round + 1
}

func initCreds(m *config) {
	// map of username list to cred objects
	// cred {username, password}
	// initialize map
	// if error getting row
	// fill with default pw
	// search mewConf.Creds for title
	/*

		for i, u := range usernames {
			pcrItem.Creds[u] = passwords[i]
			checks.Creds[u] = passwords[i]
		}
	*/
}

func parsePCR(team teamData, checkInput, pcrInput string) error {
	check, err := mewConf.getCheck(checkInput)
	if err != nil {
		return err
	}

	// get pcr collection
	coll := getCollection(mewConf.GetIdentifier(team.Name) + "pcr")
	findOptions := options.FindOne()
	findOptions.SetSort(bson.D{{"time", 1}})

	// make pcr index
	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": 1,
		}, Options: nil,
	}

	_, err = coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	pcrItem := pcrData{}

	// get previous pcr
	cursor := coll.FindOne(context.TODO(), bson.D{{"check", checkInput}}, findOptions)
	if err != nil {
		return err
	}
	err = cursor.Decode(&pcrItem)

	if err != nil {
		pcrItem = pcrData{
			Time:  time.Now(),
			Team:  team,
			Check: check.FetchName(),
			Creds: make(map[string]string),
		}
	}

	usernames := []string{}
	passwords := []string{}
	splitPcr := strings.Split(pcrInput, "\n")
	if len(splitPcr) == 0 || splitPcr[0] == "" || len(splitPcr) > 10000 {
		return errors.New("parsePCR: input empty or too large")
	}
	for _, p := range splitPcr {
		splitItem := strings.Split(p, ":")
		if len(splitItem) != 2 {
			return errors.New("parsePCR: at least one username was an invalid format")
		}
		usernames = append(usernames, splitItem[0])
		passwords = append(passwords, splitItem[1])
	}

	for i, u := range usernames {
		pcrItem.Creds[u] = passwords[i]
		checks.Creds[u] = passwords[i]
	}

	// ignoring deleteOne error
	coll.DeleteOne(context.TODO(), bson.D{{"check", pcrItem.Check}})

	_, err = coll.InsertOne(context.TODO(), pcrItem)
	if err != nil {
		return err
	}
	return nil
}

func groupTeamRecords(m *config) ([]teamRecord, error) {
	latestRecords := []teamRecord{}
	for _, team := range m.Team {
		coll := getCollection(m.GetIdentifier(team.Name) + "records")
		findOptions := options.FindOne()
		findOptions.SetSort(bson.D{{"time", -1}})
		mod := mongo.IndexModel{
			Keys: bson.M{
				"time": -1,
			}, Options: nil,
		}
		_, err := coll.Indexes().CreateOne(context.TODO(), mod)
		if err != nil {
			return latestRecords, err
		}
		cursor := coll.FindOne(context.TODO(), bson.D{}, findOptions)
		if err != nil {
			return latestRecords, err
		}
		record := teamRecord{}
		if err = cursor.Decode(&record); err != nil {
			return latestRecords, err
		}
		latestRecords = append(latestRecords, record)
	}
	return latestRecords, nil
}

func getStatus() ([]teamRecord, error) {
	records := []teamRecord{}
	coll := getCollection("status")
	opts := options.Find()
	cursor, err := coll.Find(context.TODO(), bson.D{}, opts)
	if err != nil {
		return records, err
	}
	if err = cursor.All(context.TODO(), &records); err != nil {
		return records, err
	}

	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Team.Name < records[j].Team.Name
	})

	for i := range records {
		records[i].Checks = sortResults(records[i].Checks)
	}

	return records, nil
}


func insertResult(newResult resultEntry) error {
	coll := getCollection(mewConf.GetIdentifier(newResult.Team.Name) + "results")
	_, err := coll.InsertOne(context.TODO(), newResult)
	if err != nil {
		return err
	}
	return nil
}

func replaceStatusRecord(newTeamRecord teamRecord) error {
	coll := getCollection("status")

	// ignoring deleteOne error
	coll.DeleteOne(context.TODO(), bson.D{{"team", newTeamRecord.Team}})

	_, err := coll.InsertOne(context.TODO(), newTeamRecord)
	if err != nil {
		return err
	}
	return nil
}

func pushTeamRecords(mux *sync.Mutex) {
	fmt.Println("pushing records")
	for i, rec := range recordStaging {
		if mewConf.Kind != "blue" {
			identifier := mewConf.GetIdentifier(rec.Team.Name)
			mux.Lock()
			if boxMap, ok := redPersists[identifier]; ok {
				rec.RedDetract -= len(boxMap)
				for i := range rec.Checks {
					rec.Checks[i].Persists = boxMap
				}
			}
			for _, boxMaps := range redPersists {
				for _, hackerTeams := range boxMaps {
					for _, hackerTeam := range hackerTeams {
						if hackerTeam == identifier {
							rec.RedContrib++
						}
					}
				}
			}
			recordStaging[i] = rec
			mux.Unlock()
		}
		coll := getCollection(mewConf.GetIdentifier(rec.Team.Name) + "records")
		_, err := coll.InsertOne(context.TODO(), rec)
		if err != nil {
			fmt.Println("[CRITICAL] error:", err)
		}
		replaceStatusRecord(rec)
		for _, c := range rec.Checks {
			insertResult(c)
		}
	}
	mux.Lock()
	prevRecords = recordStaging
	redPersists = make(map[string]map[string][]string)
	recordStaging = make(map[teamData]teamRecord)
	mux.Unlock()
}
