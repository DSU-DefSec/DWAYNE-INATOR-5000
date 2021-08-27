package main

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbName        = "dwayne"
	dbURI         = "mongodb://localhost:27017"
	mongoClient   *mongo.Client
	mongoCtx      context.Context
	timeConn      time.Time
	roundNumber   int
	recordStaging = make(map[string]teamRecord) // currently being built team records
)

const (
	empty = iota
	submitted
	graded
)

/*
dwayne-inator-5000
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
	team<index>-red
		red team penalties
*/

type resultEntry struct {
	Time          time.Time `json:"time,omitempty"`
	Team          string    `json:"team,omitempty"`
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
	Team          string        `json:"team,omitempty"`
	Round         int           `json:"round,omitempty"`
	Checks        []resultEntry `json:"checks,omitempty"`
	RedTeamPoints int           `json:"redteampoints,omitempty"`
	ServicePoints int           `json:"servicepoints,omitempty"`
	InjectPoints  int           `json:"injectpoints,omitempty"`
	SlaViolations int           `json:"slaviolations,omitempty"`
	Total         int           `json:"total,omitempty"`
}

type teamData struct {
	Identifier, Ip, Pw string
}

type injectSubmission struct {
	Time     time.Time `json:"time,omitempty"`
	Updated  time.Time `json:"updated,omitempty"`
	Team     string    `json:"team,omitempty"`
	Inject   int       `json:"inject,omitempty"`
	FileName string    `json:"filename,omitempty"`
	DiskFile string    `json:"diskfile,omitempty"`
	Invalid  bool      `json:"invalid,omitempty"`
	Score    int       `json:"score,omitempty"`
	Feedback string    `json:"feedback,omitempty"`
}

type injectData struct {
	Time   time.Time `json:"time,omitempty"`
	Due    time.Time `json:"due,omitempty"`
	Closes time.Time `json:"closes,omitempty"`
	Title  string    `json:"title,omitempty"`
	Body   string    `json:"body,omitempty"`
	Files  []string  `json:"files,omitempty"`
	Points int       `json:"points,omitempty"`
	Status int       `json:"status,omitempty"`
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
		debugPrint("Refreshing mongodb connection...")
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
	coll := getCollection(team.Identifier + "results")

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

func getAllTeamPCRItems() ([]checks.PcrData, error) {
	allPcrItems := []checks.PcrData{}
	for _, team := range dwConf.Team {
		pcrItems := []checks.PcrData{}
		coll := getCollection(team.Identifier + "pcr")

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
		cursor, err = coll.Find(context.TODO(), bson.D{}, findOptions)

		if err != nil {
			return pcrItems, err
		}

		if err := cursor.All(mongoCtx, &pcrItems); err != nil {
			return pcrItems, err
		}
		allPcrItems = append(allPcrItems, pcrItems...)
	}
	return allPcrItems, nil
}

func getPCRItems(team teamData, check checks.Check) ([]checks.PcrData, error) {
	pcrItems := []checks.PcrData{}
	coll := getCollection(team.Identifier + "pcr")

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

	return pcrItems, nil
}

func getTeamRecords(team string, limit int) ([]teamRecord, error) {
	records := []teamRecord{}
	coll := getCollection(team + "records")

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

func initInjects() error {
	coll := getCollection("injects")

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"time", -1}})

	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}

	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	cursor, err := coll.Find(context.TODO(), bson.D{}, findOptions)
	if err != nil {
		return err
	}

	if err := cursor.All(mongoCtx, &injects); err != nil {
		return err
	}

	return nil
}

func initStatus() error {
	coll := getCollection("status")
	err := coll.Drop(mongoCtx)
	if err != nil {
		return errors.New("error dropping collection status")
	}
	topBoard, err := groupTeamRecords(dwConf)
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
	coll := getCollection(dwConf.Team[0].Identifier + "results")
	findOptions := options.FindOne()
	findOptions.SetSort(bson.D{{"time", -1}})
	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}
	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		debugPrint("initRoundNumber:", err)
		roundNumber = 0
		return
	}
	err = coll.FindOne(context.TODO(), bson.D{}, findOptions).Decode(&res)
	if err != nil {
		debugPrint("initRoundNumber:", err)
		roundNumber = 0
		return
	}
	roundNumber = res.Round + 1
}

func initCreds() {
	allCreds := []checks.PcrData{}
	for _, team := range dwConf.Team {
		creds := []checks.PcrData{}
		coll := getCollection(team.Identifier + "pcr")
		opts := options.Find()
		cursor, err := coll.Find(context.TODO(), bson.D{}, opts)
		if err != nil {
			debugPrint("[CREDS]", err.Error())
		}
		if err = cursor.All(context.TODO(), &creds); err != nil {
			debugPrint("[CREDS]", err.Error())
		}
		allCreds = append(allCreds, creds...)
	}
	checks.Creds = allCreds
}

func parsePCR(team teamData, checkInput, pcrInput string) error {
	check, err := dwConf.getCheck(checkInput)
	if err != nil {
		return err
	}

	// get pcr collection
	coll := getCollection(team.Identifier + "pcr")
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

	pcrItem := checks.PcrData{}

	// get previous pcr
	cursor := coll.FindOne(context.TODO(), bson.D{{"check", checkInput}}, findOptions)
	err = cursor.Decode(&pcrItem)

	// if not found, create new struct
	if err != nil {
		pcrItem = checks.PcrData{
			Team:  team.Identifier,
			Check: check.FetchName(),
			Creds: make(map[string]string),
		}
	}

	// update pcr time
	pcrItem.Time = time.Now()

	// add each username/password to the map
	usernames := []string{}
	passwords := []string{}
	splitPcr := strings.Split(pcrInput, "\n")
	if len(splitPcr) == 0 || splitPcr[0] == "" || len(splitPcr) > 1000 {
		return errors.New("parsePCR: input empty or too large")
	}

	allUsernames := []string{}
	for _, cred := range dwConf.Creds {
		allUsernames = append(allUsernames, cred.Usernames...)
	}

	empty := true
	for _, p := range splitPcr {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		splitItem := strings.Split(p, ",")
		if len(splitItem) != 2 {
			return errors.New("parsePCR: username was an invalid format: " + p)
		}

		if splitItem[1] == "" {
			continue
		}

		empty = false

		if splitItem[0] == "all" {
			for _, user := range allUsernames {
				usernames = append(usernames, user)
				passwords = append(passwords, splitItem[1])
			}
		} else {
			validUser := false
			for _, user := range allUsernames {
				if user == splitItem[0] {
					validUser = true
					break
				}
			}

			if !validUser {
				continue
			}

			usernames = append(usernames, splitItem[0])
			passwords = append(passwords, splitItem[1])
		}
	}

	if empty {
		return errors.New("parsePCR: empty submission")
	}

	// add creds to pcrItem
	for i, u := range usernames {
		pcrItem.Creds[u] = passwords[i]
	}
	updateVolatilePCR(pcrItem)

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
		coll := getCollection(team.Identifier + "records")
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
		return records[i].Team < records[j].Team
	})

	for i := range records {
		records[i].Checks = sortResults(records[i].Checks)
	}

	return records, nil
}

func groupSubmissions(m *config, injectId int) ([]injectSubmission, error) {
	allSubmissions := []injectSubmission{}
	for _, team := range m.Team {
		inj, err := getSubmissions(team.Identifier, injectId)
		if err != nil {
			errorPrint(err)
		} else {
			allSubmissions = append(allSubmissions, inj...)
		}
	}
	return allSubmissions, nil
}

func getSubmission(team string, injectId int, diskFile string) (injectSubmission, error) {
	submission := injectSubmission{}
	coll := getCollection(team + "injects")

	findOptions := options.FindOne()
	findOptions.SetSort(bson.D{{"time", -1}})

	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}

	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return submission, err
	}

	err = coll.FindOne(context.TODO(), bson.D{{"inject", injectId}, {"diskfile", diskFile}}, findOptions).Decode(&submission)
	if err != nil {
		return submission, err
	}

	return submission, nil
}

func getSubmissions(team string, injectId int) ([]injectSubmission, error) {
	submissions := []injectSubmission{}
	coll := getCollection(team + "injects")

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"time", -1}})

	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}

	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return submissions, err
	}

	cursor, err := coll.Find(context.TODO(), bson.D{{"inject", injectId}}, findOptions)
	if err != nil {
		return submissions, err
	}

	if err := cursor.All(mongoCtx, &submissions); err != nil {
		return submissions, err
	}

	return submissions, nil
}

func addInject(newInject injectData) error {
	coll := getCollection("injects")
	_, err := coll.InsertOne(context.TODO(), newInject)
	if err != nil {
		return err
	}
	return nil
}

func insertSubmission(newSubmission injectSubmission) error {
	coll := getCollection(newSubmission.Team + "injects")
	_, err := coll.InsertOne(context.TODO(), newSubmission)
	if err != nil {
		return err
	}
	return nil
}

func updateSubmission(submission injectSubmission) error {
	coll := getCollection(submission.Team + "injects")

	// ignoring deleteOne error
	coll.DeleteOne(context.TODO(), bson.D{{"inject", submission.Inject}, {"diskfile", submission.DiskFile}})

	_, err := coll.InsertOne(context.TODO(), submission)
	if err != nil {
		return err
	}
	return nil
}

func insertResult(newResult resultEntry) error {
	coll := getCollection(newResult.Team + "results")
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
	debugPrint("pushing records")
	for i, rec := range recordStaging {
		coll := getCollection(rec.Team + "records")
		_, err := coll.InsertOne(context.TODO(), rec)
		if err != nil {
			errorPrint("[CRITICAL] error:", err)
		}
		replaceStatusRecord(rec)
		for _, c := range rec.Checks {
			insertResult(c)
		}
		mux.Lock()
		recordStaging[i] = rec
		mux.Unlock()
	}
}
