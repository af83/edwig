package siri

import (
	"bytes"
	"text/template"
	"time"
)

type SIRINotifyEstimatedTimeTable struct {
	Address                   string
	RequestMessageRef         string
	ProducerRef               string
	ResponseMessageIdentifier string
	SubscriberRef             string
	SubscriptionIdentifier    string

	ResponseTimestamp time.Time
	Status            bool
	ErrorType         string
	ErrorNumber       int
	ErrorText         string

	EstimatedJourneyVersionFrames []*SIRIEstimatedJourneyVersionFrame
}

const estimatedTimeTablenotifyTemplate = `<sw:NotifyEstimatedTimetable xmlns:sw="http://wsdl.siri.org.uk" xmlns:siri="http://www.siri.org.uk/siri">
	<ServiceDeliveryInfo>
		<siri:ResponseTimestamp>{{.ResponseTimestamp.Format "2006-01-02T15:04:05.000Z07:00"}}</siri:ResponseTimestamp>
		<siri:ProducerRef>{{.ProducerRef}}</siri:ProducerRef>{{ if .Address }}
		<siri:Address>{{ .Address }}</siri:Address>{{ end }}
		<siri:ResponseMessageIdentifier>{{.ResponseMessageIdentifier}}</siri:ResponseMessageIdentifier>
		<siri:RequestMessageRef>{{ .RequestMessageRef }}</siri:RequestMessageRef>
	</ServiceDeliveryInfo>
	<Notification>
		<siri:EstimatedTimetableDelivery version="2.0:FR-IDF-2.4">
			<siri:ResponseTimestamp>{{.ResponseTimestamp.Format "2006-01-02T15:04:05.000Z07:00"}}</siri:ResponseTimestamp>
			<siri:RequestMessageRef>{{.RequestMessageRef}}</siri:RequestMessageRef>
			<siri:SubscriberRef>{{.SubscriberRef}}</siri:SubscriberRef>
			<siri:SubscriptionRef>{{.SubscriptionIdentifier}}</siri:SubscriptionRef>
			<siri:Status>{{ .Status }}</siri:Status>{{ if not .Status }}
			<siri:ErrorCondition>{{ if eq .ErrorType "OtherError" }}
				<siri:OtherError number="{{.ErrorNumber}}">{{ else }}
				<siri:{{.ErrorType}}>{{ end }}
					<siri:ErrorText>{{.ErrorText}}</siri:ErrorText>
				</siri:{{.ErrorType}}>
			</siri:ErrorCondition>{{ else }}{{ range .EstimatedJourneyVersionFrames }}
			{{ .BuildEstimatedJourneyVersionFrameXML }}{{ end }}{{ end }}
		</siri:EstimatedTimetableDelivery>
	</Notification>
	<NotificationExtension/>
</sw:NotifyEstimatedTimetable>`

func (notify *SIRINotifyEstimatedTimeTable) BuildXML() (string, error) {
	var buffer bytes.Buffer
	var notifyDelivery = template.Must(template.New("estimatedTimeTableNotify").Parse(estimatedTimeTablenotifyTemplate))
	if err := notifyDelivery.Execute(&buffer, notify); err != nil {
		return "", err
	}
	return buffer.String(), nil
}
