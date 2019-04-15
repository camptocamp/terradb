package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	//log "github.com/sirupsen/logrus"

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

type document struct {
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
func (st *MongoDBStorage) GetLockStatus(name string) (lockStatus interface{}, err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	var data map[string]interface{}

	res := collection.FindOne(ctx, map[string]interface{}{
		"name": name,
	})
	if res.Err() != nil {
		err = res.Err()
		return
	}
	err = res.Decode(&data)
	// Assume no document is returned
	if err != nil {
		err = nil
		return
	}
	if data == nil {
		return
	}

	lockStatus, ok := data["lock"].(interface{})
	if !ok {
		err = fmt.Errorf("lock info not found")
	}

	return
}

// LockState locks a Terraform state.
func (st *MongoDBStorage) LockState(name string, lockData interface{}) (err error) {
	collection := st.client.Database("terradb").Collection("locks")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	_, err = collection.InsertOne(ctx, map[string]interface{}{
		"name": name,
		"lock": lockData,
	})

	return
}

// UnlockState unlocks a Terraform state.
func (st *MongoDBStorage) UnlockState(name string, lockData interface{}) (err error) {
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

// GetState retrieve the latest version of a Terraform state.
func (st *MongoDBStorage) GetState(name string, version int) (document interface{}, err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	var data map[string]interface{}

	err = collection.FindOne(ctx, map[string]interface{}{
		"name": name,
	}, &options.FindOneOptions{
		Sort: map[string]interface{}{
			"state.serial": version,
		},
	}).Decode(&data)
	if err != nil {
		return
	}

	document, ok := data["state"].(interface{})
	if !ok {
		err = fmt.Errorf("state file not found")
	}
	return
}

// InsertState adds a Terraform state to the database.
func (st *MongoDBStorage) InsertState(doc interface{}, timestamp, source, name string) (err error) {
	collection := st.client.Database("terradb").Collection("terraform_states")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	v, ok := doc.(map[string]interface{})
	if !ok {
		err = fmt.Errorf("failed to unmarshal document")
		return
	}

	serial, ok := v["serial"].(int)
	if !ok {
		serial = 0
	}

	var query interface{}
	json.Unmarshal([]byte(fmt.Sprintf(`{
		"state.serial": "%v",
		"name": "%s"
	}`, serial, name)), &query)

	data := &document{
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
