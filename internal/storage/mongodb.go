package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoDBConfig stores the informations required to connect a MongoDB database.
type MongoDBConfig struct {
	URL      string
	Username string
	Password string
}

// MongoDBStorage stores the MongoDB client.
type MongoDBStorage struct {
	client *mongo.Client
}

type mongoDoc struct {
	Timestamp string
	Source    string
	State     *State
	Name      string
}

// a collection of paginated mongoDoc
type mongoDocCollection struct {
	Metadata []*Metadata
	Docs     []*mongoDoc
}

type mongoLockInfoDoc struct {
	Name string   `json:"name"`
	Lock LockInfo `json:"lock"`
}

// NewMongoDB initializes a connection to the defined MongoDB instance.
func NewMongoDB(config *MongoDBConfig) (st *MongoDBStorage, err error) {
	st = &MongoDBStorage{}

	clientOptions := options.Client()

	if config.Username != "" {
		clientOptions.SetAuth(options.Credential{
			Username: config.Username,
			Password: config.Password,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st.client, err = mongo.Connect(ctx, clientOptions.ApplyURI(config.URL))
	if err != nil {
		return
	}
	err = st.client.Ping(ctx, readpref.Primary())
	return
}

// GetName returns the storage's name.
func (*MongoDBStorage) GetName() string {
	return "mongodb"
}

// GetLockStatus returns a Terraform lock.
func (st *MongoDBStorage) GetLockStatus(name string) (lockStatus LockInfo, err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := collection.FindOne(ctx, bson.M{"name": name})
	if res.Err() != nil {
		err = res.Err()
		return
	}
	var lockDoc mongoLockInfoDoc
	err = res.Decode(&lockDoc)
	// Assume no document is returned
	if err != nil {
		return lockStatus, ErrNoDocuments
	}
	lockStatus = lockDoc.Lock

	return
}

// LockState locks a Terraform state.
func (st *MongoDBStorage) LockState(name string, lockData LockInfo) (err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// State file uses the same key as lock
	lockData.Path = name

	_, err = collection.InsertOne(ctx, map[string]interface{}{
		"name": name,
		"lock": lockData,
	})

	return
}

// UnlockState unlocks a Terraform state.
func (st *MongoDBStorage) UnlockState(name string, lockData LockInfo) (err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = collection.DeleteOne(ctx, map[string]interface{}{
		"name": name,
	}, &options.DeleteOptions{})

	return
}

// RemoveState removes the Terraform states.
func (st *MongoDBStorage) RemoveState(name string) (err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = collection.DeleteOne(ctx, map[string]interface{}{
		"name": name,
	}, &options.DeleteOptions{})

	return
}

// ListStates returns all state names from TerraDB
func (st *MongoDBStorage) ListStates(pageNum, pageSize int) (coll StateCollection, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := mongo.Pipeline{
		{{"$group", bson.D{
			{"_id", "$name"},
			{"name", bson.D{{"$last", "$name"}}},
			{"state", bson.D{{"$last", "$state"}}},
			{"timestamp", bson.D{{"$last", "$timestamp"}}},
		}}},
	}
	pl := paginateReq(req, pageNum, pageSize)
	cur, err := collection.Aggregate(ctx, pl, options.Aggregate())
	if err != nil {
		return coll, fmt.Errorf("failed to list states: %v", err)
	}

	defer cur.Close(context.Background())

	for cur.Next(nil) {
		var mongoColl mongoDocCollection
		err = cur.Decode(&mongoColl)
		if err != nil {
			return coll, fmt.Errorf("failed to decode states: %v", err)
		}
		coll.Metadata = mongoColl.Metadata
		for _, d := range mongoColl.Docs {
			state, err := d.toState()
			if err != nil {
				return coll, fmt.Errorf("failed to get state: %v", err)
			}

			state.LockInfo, err = st.GetLockStatus(state.Name)
			// Init value required because of omitempty
			state.Locked = false
			if err == nil {
				state.Locked = true
			} else if err == ErrNoDocuments {
				log.WithFields(log.Fields{
					"name": state.Name,
				}).Info("Did not find lock info")
			} else {
				return coll, fmt.Errorf("failed to retrieve lock for %s: %v", state.Name, err)
			}
			coll.Data = append(coll.Data, state)
		}
		return coll, nil
	}

	return coll, nil
}

// GetState retrieves a Terraform state, at a given serial.
// If serial is 0, it gets the latest serial
func (st *MongoDBStorage) GetState(name string, serial int) (state State, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := map[string]interface{}{
		"name": name,
	}

	if serial != 0 {
		filter["state.serial"] = serial
	}

	var doc mongoDoc
	err = collection.FindOne(
		ctx, filter,
		options.FindOne().SetSort(bson.M{"state.serial": -1}),
	).Decode(&doc)

	if err == mongo.ErrNoDocuments {
		err = ErrNoDocuments
		return
	} else if err != nil {
		err = fmt.Errorf("failed to decode state: %v", err)
		return
	}

	s, err := doc.toState()
	if err != nil {
		return state, fmt.Errorf("failed to get state: %v", err)
	}
	state = *s
	// Init value required because of omitempty
	state.Locked = false
	state.LockInfo, err = st.GetLockStatus(state.Name)
	if err == nil {
		state.Locked = true
	} else if err == ErrNoDocuments {
		log.WithFields(log.Fields{
			"name": state.Name,
		}).Info("Did not find lock info")
		// Reset err
		err = nil
	} else {
		return state, fmt.Errorf("failed to retrieve lock for %s: %v", state.Name, err)
	}
	return
}

// InsertState adds a Terraform state to the database.
func (st *MongoDBStorage) InsertState(doc State, timestamp, source, name string) (err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var query interface{}
	json.Unmarshal([]byte(fmt.Sprintf(`{
		"state.serial": "%v",
		"name": "%s"
	}`, doc.Serial, name)), &query)

	data := &mongoDoc{
		Timestamp: timestamp,
		Source:    source,
		Name:      name,
		State:     &doc,
	}

	upsert := true

	_, err = collection.UpdateOne(ctx, query, map[string]interface{}{
		"$set": data,
	}, &options.UpdateOptions{
		Upsert: &upsert,
	})

	return
}

// ListStateSerials returns all state serials with a given name.
func (st *MongoDBStorage) ListStateSerials(name string, pageNum, pageSize int) (coll StateCollection, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := mongo.Pipeline{
		{{"$match", bson.D{{"name", name}}}},
		{{"$sort", bson.D{{"state.serial", 1}}}},
	}

	pl := paginateReq(req, pageNum, pageSize)
	cur, err := collection.Aggregate(ctx, pl, options.Aggregate())
	if err != nil {
		return coll, fmt.Errorf("failed to list states: %v", err)
	}

	defer cur.Close(context.Background())

	var mongoColl mongoDocCollection
	for cur.Next(nil) {
		err = cur.Decode(&mongoColl)
		if err != nil {
			return coll, fmt.Errorf("failed to decode states: %v", err)
		}
		coll.Metadata = mongoColl.Metadata
		for _, d := range mongoColl.Docs {
			state, err := d.toState()
			if err != nil {
				return coll, fmt.Errorf("failed to get state: %v", err)
			}
			coll.Data = append(coll.Data, state)
		}
		if err != nil {
			return coll, fmt.Errorf("failed to list states: %v", err)
		}
		return
	}

	return
}

// GetResource retrieves a Terraform resource given a state, module and name
func (st *MongoDBStorage) GetResource(state, module, name string) (res Resource, err error) {
	s, err := st.GetState(state, 0)
	if err != nil {
		return res, fmt.Errorf("failed to get resource: %v", err)
	}

	res, err = getResource(s, module, name)
	return
}

func getResource(state State, module, name string) (res Resource, err error) {
	for _, m := range state.Modules {
		for _, p := range m.Path {
			if p == module {
				r, ok := m.Resources[name]
				if ok {
					return *r, nil
				}
				return res, ErrNoDocuments
			}
		}
	}
	return res, ErrNoDocuments
}

func paginateReq(req mongo.Pipeline, pageNum, pageSize int) (pl mongo.Pipeline) {
	skips := pageSize * (pageNum - 1)

	pl = append(req,
		bson.D{{"$facet", bson.D{
			{"metadata", bson.A{
				bson.D{{"$count", "total"}},
				bson.D{{"$addFields", bson.D{{"page", pageNum}}}},
			}},
			{"docs", bson.A{
				bson.D{{"$skip", skips}},
				bson.D{{"$limit", pageSize}},
			}},
		},
		}},
	)

	return
}

func (d *mongoDoc) toState() (state *State, err error) {
	state = d.State
	state.Name = d.Name
	state.LastModified, err = time.Parse("20060102150405", d.Timestamp)
	if err != nil {
		return state, fmt.Errorf("failed to convert timestamp: %v", err)
	}
	return
}
