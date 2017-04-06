package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/af83/edwig/core"
	"github.com/af83/edwig/model"
)

func checkSituationResponseStatus(responseRecorder *httptest.ResponseRecorder, t *testing.T) {
	if status := responseRecorder.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code:\n got %v\n want %v",
			status, http.StatusOK)
	}

	if contentType := responseRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Handler returned wrong Content-Type:\n got: %v\n want: %v",
			contentType, "application/json")
	}
}

func prepareSituationRequest(method string, sendIdentifier bool, body []byte, t *testing.T) (situation model.Situation, responseRecorder *httptest.ResponseRecorder, referential *core.Referential) {
	// Create a referential
	referentials := core.NewMemoryReferentials()
	server := &Server{}
	server.SetReferentials(referentials)
	referential = referentials.New("default")
	referential.Save()

	// Set the fake UUID generator
	model.SetDefaultUUIDGenerator(model.NewFakeUUIDGenerator())
	// Save a new situation
	situation = referential.Model().Situations().New()
	referential.Model().Situations().Save(&situation)

	// Create a request
	address := []byte("/default/situations")
	if sendIdentifier {
		address = append(address, fmt.Sprintf("/%s", situation.Id())...)
	}
	request, err := http.NewRequest(method, string(address), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder
	responseRecorder = httptest.NewRecorder()

	// Call APIHandler method and pass in our Request and ResponseRecorder.
	server.APIHandler(responseRecorder, request)

	return
}

func Test_SituationController_Delete(t *testing.T) {
	// Send request
	situation, responseRecorder, referential := prepareSituationRequest("DELETE", true, nil, t)

	// Test response
	checkSituationResponseStatus(responseRecorder, t)

	//Test Results
	_, ok := referential.Model().Situations().Find(situation.Id())
	if ok {
		t.Errorf("Situation shouldn't be found after DELETE request")
	}
	if expected, _ := situation.MarshalJSON(); responseRecorder.Body.String() != string(expected) {
		t.Errorf("Wrong body for DELETE response request:\n got: %v\n want: %v", responseRecorder.Body.String(), string(expected))
	}
}

func Test_SituationController_Update(t *testing.T) {
	// Prepare and send request
	body := []byte(`{ "ObjectIDs": { "reflex": "FR:77491:ZDE:34004:STIF" } }`)
	situation, responseRecorder, referential := prepareSituationRequest("PUT", true, body, t)

	// Check response
	checkSituationResponseStatus(responseRecorder, t)

	// Test Results
	updatedSituation, ok := referential.Model().Situations().Find(situation.Id())
	if !ok {
		t.Errorf("Situation should be found after PUT request")
	}

	if expected, _ := updatedSituation.MarshalJSON(); responseRecorder.Body.String() != string(expected) {
		t.Errorf("Wrong body for PUT response request:\n got: %v\n want: %v", responseRecorder.Body.String(), string(expected))
	}
}

func Test_SituationController_Show(t *testing.T) {
	// Send request
	situation, responseRecorder, _ := prepareSituationRequest("GET", true, nil, t)

	// Test response
	checkSituationResponseStatus(responseRecorder, t)

	//Test Results
	if expected, _ := situation.MarshalJSON(); responseRecorder.Body.String() != string(expected) {
		t.Errorf("Wrong body for GET (show) response request:\n got: %v\n want: %v", responseRecorder.Body.String(), string(expected))
	}
}

func Test_SituationController_Create(t *testing.T) {
	// Prepare and send request
	body := []byte(`{ "Reference" : {"ObjectId":{"lol":"lel"}, "Id":"42"},
		"ObjectIDs": { "reflex": "FR:77491:ZDE:34004:STIF" } }`)
	_, responseRecorder, referential := prepareSituationRequest("POST", false, body, t)

	// Check response
	checkSituationResponseStatus(responseRecorder, t)

	// Test Results
	// Using the fake uuid generator, the uuid of the created
	// situation should be 6ba7b814-9dad-11d1-1-00c04fd430c8
	situation, ok := referential.Model().Situations().Find("6ba7b814-9dad-11d1-1-00c04fd430c8")
	if !ok {
		t.Errorf("Situation should be found after POST request")
	}
	situationMarshal, _ := situation.MarshalJSON()
	expected := `{"Id":"6ba7b814-9dad-11d1-1-00c04fd430c8","ObjectIDs":{"reflex":"FR:77491:ZDE:34004:STIF"},"Reference":{"ObjectId":{"lol":"lel"},"Id":"42"}}`
	if responseRecorder.Body.String() != string(expected) && string(situationMarshal) != string(expected) {
		t.Errorf("Wrong body for POST response request:\n got: %v\n want: %v", responseRecorder.Body.String(), string(expected))
	}
}

func Test_SituationController_Index(t *testing.T) {
	// Send request
	_, responseRecorder, _ := prepareSituationRequest("GET", false, nil, t)

	// Test response
	checkSituationResponseStatus(responseRecorder, t)

	//Test Results
	expected := `[{"Id":"6ba7b814-9dad-11d1-0-00c04fd430c8"}]`
	if responseRecorder.Body.String() != string(expected) {
		t.Errorf("Wrong body for GET (index) response request:\n got: %v\n want: %v", responseRecorder.Body.String(), string(expected))
	}
}