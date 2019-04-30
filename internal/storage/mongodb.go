package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	//log "github.com/sirupsen/logrus"

	"github.com/hashicorp/terraform/state"
	"github.com/hashicorp/terraform/terraform"
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
	State     interface{}
	Name      string
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

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
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
func (st *MongoDBStorage) GetLockStatus(name string) (lockStatus state.LockInfo, err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	res := collection.FindOne(ctx, bson.M{"name": name})
	if res.Err() != nil {
		err = res.Err()
		return
	}
	err = res.Decode(&lockStatus)
	// Assume no document is returned
	if err != nil {
		return lockStatus, ErrNoDocuments
	}

	return
}

// LockState locks a Terraform state.
func (st *MongoDBStorage) LockState(name string, lockData state.LockInfo) (err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	_, err = collection.InsertOne(ctx, map[string]interface{}{
		"name": name,
		"lock": lockData,
	})

	return
}

// UnlockState unlocks a Terraform state.
func (st *MongoDBStorage) UnlockState(name string, lockData state.LockInfo) (err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	_, err = collection.DeleteOne(ctx, map[string]interface{}{
		"name": name,
	}, &options.DeleteOptions{})

	return
}

// RemoveState removes the Terraform states.
func (st *MongoDBStorage) RemoveState(name string) (err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	_, err = collection.DeleteOne(ctx, map[string]interface{}{
		"name": name,
	}, &options.DeleteOptions{})

	return
}

// ListStates returns all state names from TerraDB
func (st *MongoDBStorage) ListStates(page_num, page_size int) (coll DocumentCollection, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	req := mongo.Pipeline{
		{{"$group", bson.D{
			{"_id", "$name"},
			{"name", bson.D{{"$last", "$name"}}},
			{"state", bson.D{{"$last", "$state"}}},
			{"timestamp", bson.D{{"$last", "$timestamp"}}},
		}}},
	}
	pl := paginateReq(req, page_num, page_size)
	cur, err := collection.Aggregate(ctx, pl, options.Aggregate())
	if err != nil {
		return coll, fmt.Errorf("failed to list states: %v", err)
	}

	defer cur.Close(context.Background())

	for cur.Next(nil) {
		err = cur.Decode(&coll)
		if err != nil {
			return coll, fmt.Errorf("failed to decode states: %v", err)
		}
		for _, s := range coll.Data {
			s.LastModified, err = time.Parse("20060102150405", s.Timestamp)
			if err != nil {
				return coll, fmt.Errorf("failed to convert timestamp: %v", err)
			}
		}
		return
	}

	return
}

// GetState retrieves a Terraform state, at a given serial.
// If serial is 0, it gets the latest serial
func (st *MongoDBStorage) GetState(name string, serial int) (state terraform.State, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	filter := map[string]interface{}{
		"name": name,
	}

	if serial != 0 {
		filter["state.serial"] = serial
	}

	var doc Document
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

	state = *doc.State
	return
}

// InsertState adds a Terraform state to the database.
func (st *MongoDBStorage) InsertState(doc terraform.State, timestamp, source, name string) (err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	var query interface{}
	json.Unmarshal([]byte(fmt.Sprintf(`{
		"state.serial": "%v",
		"name": "%s"
	}`, doc.Serial, name)), &query)

	data := &mongoDoc{
		Timestamp: timestamp,
		Source:    source,
		Name:      name,
		State:     doc,
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
func (st *MongoDBStorage) ListStateSerials(name string, page_num, page_size int) (coll DocumentCollection, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	req := mongo.Pipeline{
		{{"$match", bson.D{{"name", name}}}},
		{{"$sort", bson.D{{"state.serial", 1}}}},
	}

	pl := paginateReq(req, page_num, page_size)
	cur, err := collection.Aggregate(ctx, pl, options.Aggregate())
	if err != nil {
		return coll, fmt.Errorf("failed to list states: %v", err)
	}

	defer cur.Close(context.Background())

	for cur.Next(nil) {
		err = cur.Decode(&coll)
		if err != nil {
			return coll, fmt.Errorf("failed to decode states: %v", err)
		}
		for _, s := range coll.Data {
			s.LastModified, err = time.Parse("20060102150405", s.Timestamp)
			if err != nil {
				return coll, fmt.Errorf("failed to convert timestamp: %v", err)
			}
		}
		return
	}

	return
}

func paginateReq(req mongo.Pipeline, page_num, page_size int) (pl mongo.Pipeline) {
	skips := page_size * (page_num - 1)

	pl = append(req,
		bson.D{{"$facet", bson.D{
			{"metadata", bson.A{
				bson.D{{"$count", "total"}},
				bson.D{{"$addFields", bson.D{{"page", page_num}}}},
			}},
			{"data", bson.A{
				bson.D{{"$skip", skips}},
				bson.D{{"$limit", page_size}},
			}},
		},
		}},
	)

	return
}
