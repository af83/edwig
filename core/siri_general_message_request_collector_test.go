package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/af83/edwig/audit"
	"github.com/af83/edwig/model"
)

func prepare_SIRIGeneralMessageRequestCollector(t *testing.T, responseFilePath string) []*model.SituationUpdateEvent {
	audit.SetCurrentLogstash(audit.NewFakeLogStash())
	// Create a test http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength <= 0 {
			t.Errorf("Request Content Length should be zero")
		}
		file, err := os.Open(responseFilePath)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		io.Copy(w, file)
	}))
	defer ts.Close()

	partners := createTestPartnerManager()
	partner := &Partner{
		context: make(Context),
		Settings: map[string]string{
			"remote_url":           ts.URL,
			"remote_objectid_kind": "test kind",
		},
		manager: partners,
	}
	partners.Save(partner)

	situation := partners.Model().Situations().New()
	objectid := model.NewObjectID("test kind", "test value")
	situation.SetObjectID(objectid)
	partners.Model().Situations().Save(&situation)

	siriGeneralMessageRequestCollector := NewSIRIGeneralMessageRequestCollector(partner)
	siriGeneralMessageRequestCollector.SetClock(model.NewFakeClock())
	situationUpdateRequest := NewSituationUpdateRequest(SituationUpdateRequestId(situation.Id()))
	situationUpdateEvents, err := siriGeneralMessageRequestCollector.RequestSituationUpdate(situationUpdateRequest)
	if err != nil {
		t.Fatal(err)
	}
	return situationUpdateEvents
}

func Test_SIRIGeneralMessageCollectorFactory_Validate(t *testing.T) {
	partner := &Partner{
		slug:           "partner",
		Settings:       make(map[string]string),
		ConnectorTypes: []string{"siri-general-message-request-collector"},
		connectors:     make(map[string]Connector),
		manager:        NewPartnerManager(nil),
	}
	apiPartner := partner.Definition()
	apiPartner.Validate()
	if apiPartner.Errors.Empty() {
		t.Errorf("apiPartner should have three errors when remote_url and remote_objectid_kind aren't set, got: %v", apiPartner.Errors)
	}

	apiPartner.Settings = map[string]string{
		"remote_url":           "remote_url",
		"remote_objectid_kind": "remote_objectid_kind",
		"remote_credential":    "remote_credential",
	}
	apiPartner.Validate()
	if !apiPartner.Errors.Empty() {
		t.Errorf("apiPartner shouldn't have any error when remote_url is set, got: %v", apiPartner.Errors)
	}
}

func Test_SIRIGeneralMessageRequestCollector_RequestSituationUpdate(t *testing.T) {
	situationUpdateEvents := prepare_SIRIGeneralMessageRequestCollector(t, "testdata/generalmessage-response-soap.xml")
	if situationUpdateEvents == nil {
		t.Error("RequestSituationUpdate should not return nil")
	}

	if len(situationUpdateEvents) != 2 {
		t.Errorf("RequestSituationUpdate should have 2 SituationUpdateEvents, got: %v", len(situationUpdateEvents))
	}
	situationEvent := situationUpdateEvents[0]

	if expected := model.FAKE_CLOCK_INITIAL_DATE; situationEvent.CreatedAt != expected {
		t.Errorf("Wrong Created_At for situationEvent:\n expected: %v\n got: %v", expected, situationEvent.CreatedAt)
	}
	if expected, _ := time.Parse(time.RFC3339, "2017-03-29T03:30:06.000+02:00"); !situationEvent.RecordedAt.Equal(expected) {
		t.Errorf("Wrong RecorderAt for situationEvent:\n expected: %v\n got: %v", expected, situationEvent.RecordedAt)
	}

	if expected := 1; situationEvent.Version != int64(expected) {
		t.Errorf("Wrong Version for situationEvent:\n expected: %v\n got: %v", expected, situationEvent.Version)
	}

	if expected := "Commercial"; situationEvent.SituationAttributes.Channel != expected {
		t.Errorf("Wrong Channel for situationEvent:\n expected: %v\n got: %v", expected, situationEvent.SituationAttributes.Channel)
	}

	if expected, _ := time.Parse(time.RFC3339, "2017-03-29T20:30:06.000+02:00"); !situationEvent.SituationAttributes.ValidUntil.Equal(expected) {
		t.Errorf("Wrong ValidUntil for situationEvent:\n expected: %v\n got: %v", expected, situationEvent.SituationAttributes.ValidUntil)
	}

	if expected := "NINOXE:default"; situationEvent.ProducerRef != "NINOXE:default" {
		t.Errorf("Wrong ProducerRef for situationEvent:\n expected: %v\n got: %v", expected, situationEvent.ProducerRef)
	}

	messages := situationEvent.SituationAttributes.Messages

	if len(messages) != 2 {
		t.Error("messages length should be 2")
	}
	if expected := "longMessage"; messages[0].Type != expected {
		t.Errorf("Wrong message type got: %v want: %v", messages[0].Type, expected)
	}

	if expected := "test"; messages[0].Content != expected {
		t.Errorf("Wrong message type got: %v want: %v", messages[0].Content, expected)
	}
}
