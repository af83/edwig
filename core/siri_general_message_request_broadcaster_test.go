package core

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/af83/edwig/model"
	"github.com/af83/edwig/siri"
)

func Test_SIRIGeneralMessageRequestBroadcaster_RequestStopArea(t *testing.T) {
	referentials := NewMemoryReferentials()
	referential := referentials.New("referential")
	partner := referential.Partners().New("partner")
	partner.Settings["local_url"] = "http://edwig"
	partner.Settings["remote_objectid_kind"] = "objectidKind"

	connector := NewSIRIGeneralMessageRequestBroadcaster(partner)
	mid := NewFormatMessageIdentifierGenerator("Edwig:ResponseMessage::%s:LOC")
	mid.SetUUIDGenerator(model.NewFakeUUIDGenerator())
	connector.SIRIPartner().SetResponseMessageIdentifierGenerator(mid)
	connector.SetClock(model.NewFakeClock())

	objectid := model.NewObjectID("objectidKind", "NINOXE:StopPoint:SP:24:LOC")
	situation := referential.Model().StopAreas().New()
	situation.SetObjectID(objectid)
	situation.Save()

	file, err := os.Open("testdata/generalmessage-request-soap.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	request, err := siri.NewXMLGeneralMessageRequestFromContent(content)
	if err != nil {
		t.Fatal(err)
	}

	response, _ := connector.Situations(request)

	if response.Address != "http://edwig" {
		t.Errorf("Response has wrong adress:\n got: %v\n want: http://edwig", response.Address)
	}
	if response.ProducerRef != "Edwig" {
		t.Errorf("Response has wrong producerRef:\n got: %v\n expected: Edwig", response.ProducerRef)
	}
	if response.ResponseMessageIdentifier != "Edwig:ResponseMessage::6ba7b814-9dad-11d1-0-00c04fd430c8:LOC" {
		t.Errorf("Response has wesponseMessageIdentifier:\n got: %v\n expected: RATPDev:Message::ade15433-06a6-4f7b-b331-2c1080a5d279:LOC", response.ResponseMessageIdentifier)
	}
	time := connector.Clock().Now()
	if !response.ResponseTimestamp.Equal(time) {
		t.Errorf("Response has wrong responseTimestamp:\n got: %v\n expected: 2016-09-22 08:01:20.227 +0200 CEST", response.ResponseTimestamp)
	}
}

func Test_SIRIGeneralMessageRequestBroadcasterFactory_Validate(t *testing.T) {
	partner := &Partner{
		slug:           "partner",
		Settings:       make(map[string]string),
		ConnectorTypes: []string{"siri-general-message-request-broadcaster"},
		connectors:     make(map[string]Connector),
		manager:        NewPartnerManager(nil),
	}
	apiPartner := partner.Definition()
	apiPartner.Validate()
	if apiPartner.Errors.Empty() {
		t.Errorf("apiPartner should have errors when local_credential and remote_objectid_kind aren't set, got: %v", apiPartner.Errors)
	}

	apiPartner.Settings = map[string]string{
		"remote_objectid_kind": "remote_objectid_kind",
		"local_credential":     "local_credential",
	}
	apiPartner.Validate()
	if !apiPartner.Errors.Empty() {
		t.Errorf("apiPartner shouldn't have any error when local_credential and remote_objectid_kind are set, got: %v", apiPartner.Errors)
	}
}

func Test_SIRIGeneralMessageRequestBroadcaster_RemoteObjectIDKindAbsent(t *testing.T) {
	partner := &Partner{}
	partner.Settings = make(map[string]string)

	partner.Settings["siri-general-message-request-broadcaster.remote_objectid_kind"] = ""
	partner.Settings["remote_objectid_kind"] = "Kind2"

	connector := NewSIRIGeneralMessageRequestBroadcaster(partner)

	if connector.RemoteObjectIDKind() != "Kind2" {
		t.Errorf("RemoteObjectIDKind should be egals to Kind2")
	}
}

func Test_SIRIGeneralMessageBroadcaster_RemoteObjectIDKindPresent(t *testing.T) {
	partner := &Partner{}
	partner.Settings = make(map[string]string)

	partner.Settings["siri-general-message-request-broadcaster.remote_objectid_kind"] = "Kind1"
	partner.Settings["remote_objectid_kind"] = "Kind2"

	connector := NewSIRIGeneralMessageRequestBroadcaster(partner)

	if connector.RemoteObjectIDKind() != "Kind1" {
		t.Errorf("RemoteObjectIDKind should be egals to Kind1")
	}
}