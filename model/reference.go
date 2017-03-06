package model

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
)

type Reference struct {
	ObjectId *ObjectID
	Id       string
}

func (reference *Reference) GetSha1() string {
	hasher := sha1.New() // oui, on sait
	hasher.Write([]byte(reference.ObjectId.Value()))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (reference *Reference) Getformat(ref, value string) string {
	allRef := make(map[string]string)

	allRef["PlaceRef"] = "StopPoint"
	allRef["OriginRef"] = "StopPoint"
	allRef["DestinationRef"] = "StopPoint"

	formated := "RATPDev:" + allRef[ref] + ":Q:" + value + ":LOC"
	return formated
}

func (reference *Reference) UnmarshalJSON(data []byte) error {

	aux := &struct {
		ObjectId map[string]string
		Id       string
	}{}

	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	if len(aux.ObjectId) != 1 {
		return errors.New("ObjectID should look like KIND:VALUE")
	}

	for kind, _ := range aux.ObjectId {
		ObjectIdCPY := NewObjectID(kind, aux.ObjectId[kind])
		reference.ObjectId = &ObjectIdCPY
	}

	reference.Id = aux.Id
	return nil
}
