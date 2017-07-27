package siri

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func getXMLGetEstimatedTimetableRequest(t *testing.T) *XMLGetEstimatedTimetableRequest {
	file, err := os.Open("testdata/estimated_timetable_request.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	request, _ := NewXMLGetEstimatedTimetableRequestFromContent(content)
	return request
}

func Test_XMLGetEstimatedTimetableRequest_RequestorRef(t *testing.T) {
	request := getXMLGetEstimatedTimetableRequest(t)
	if expected := "test"; request.RequestorRef() != expected {
		t.Errorf("Wrong RequestorRef:\n got: %v\nwant: %v", request.RequestorRef(), expected)
	}
}

func Test_XMLGetEstimatedTimetableRequest_RequestTimestamp(t *testing.T) {
	request := getXMLGetEstimatedTimetableRequest(t)
	if expected := time.Date(2016, time.September, 7, 9, 11, 25, 174000000, time.UTC); request.RequestTimestamp() != expected {
		t.Errorf("Wrong RequestTimestamp:\n got: %v\nwant: %v", request.RequestTimestamp(), expected)
	}
}

func Test_XMLGetEstimatedTimetableRequest_MessageIdentifier(t *testing.T) {
	request := getXMLGetEstimatedTimetableRequest(t)
	if expected := "EstimatedTimetable:Test:0"; request.MessageIdentifier() != expected {
		t.Errorf("Wrong MessageIdentifier:\n got: %v\nwant: %v", request.MessageIdentifier(), expected)
	}
}

func Test_XMLGetEstimatedTimetableRequest_Lines(t *testing.T) {
	request := getXMLGetEstimatedTimetableRequest(t)
	if len(request.Lines()) != 2 {
		t.Fatalf("GetEstimatedTimetable request has wrong number of lines: %v", request.Lines())
	}
	if expected := "NINOXE:Line:2:LOC"; request.Lines()[0] != expected {
		t.Errorf("Wrong first line:\n got: %v\nwant: %v", request.Lines()[0], expected)
	}
	if expected := "NINOXE:Line:3:LOC"; request.Lines()[1] != expected {
		t.Errorf("Wrong first line:\n got: %v\nwant: %v", request.Lines()[1], expected)
	}
}
