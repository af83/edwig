package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/af83/edwig/audit"
	"github.com/af83/edwig/logger"
	"github.com/af83/edwig/model"
	"github.com/af83/edwig/siri"
)

type StopMonitoringSubscriptionCollector interface {
	RequestStopAreaUpdate(request *StopAreaUpdateRequest)
	HandleNotifyStopMonitoring(delivery *siri.XMLStopMonitoringResponse)
}

type SIRIStopMonitoringSubscriptionCollector struct {
	model.ClockConsumer
	model.UUIDConsumer

	siriConnector

	stopAreaUpdateSubscriber StopAreaUpdateSubscriber
}

type SIRIStopMonitoringSubscriptionCollectorFactory struct{}

func (factory *SIRIStopMonitoringSubscriptionCollectorFactory) CreateConnector(partner *Partner) Connector {
	return NewSIRIStopMonitoringSubscriptionCollector(partner)
}

func (factory *SIRIStopMonitoringSubscriptionCollectorFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	ok = ok && apiPartner.ValidatePresenceOfSetting("local_credential")
	return ok
}

func NewSIRIStopMonitoringSubscriptionCollector(partner *Partner) *SIRIStopMonitoringSubscriptionCollector {
	siriStopMonitoringSubscriptionCollector := &SIRIStopMonitoringSubscriptionCollector{}
	siriStopMonitoringSubscriptionCollector.partner = partner
	manager := partner.Referential().CollectManager()
	siriStopMonitoringSubscriptionCollector.stopAreaUpdateSubscriber = manager.BroadcastStopAreaUpdateEvent

	return siriStopMonitoringSubscriptionCollector
}

func (connector *SIRIStopMonitoringSubscriptionCollector) RequestStopAreaUpdate(request *StopAreaUpdateRequest) {
	logStashEvent := make(audit.LogStashEvent)
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)

	subscription := connector.partner.Subscriptions().FindOrCreateByKind("StopMonitoring")

	for _, sr := range subscription.resourcesByObjectID {
		if sr.Reference.ObjectId.Value() == string(request.StopAreaId()) {
			sr.SubscribedUntil = sr.SubscribedUntil.Add(1 * time.Minute)
			return
		}
	}

	objId := model.NewObjectID("StopMonitoring", string(request.StopAreaId()))
	ref := model.Reference{
		ObjectId: &objId,
		Id:       string(request.StopAreaId()),
		Type:     "StopArea",
	}

	subscription.CreateAddNewResource(ref)

	siriStopMonitoringSubscriptionRequest := &siri.SIRIStopMonitoringSubscriptionRequest{
		MessageIdentifier:      connector.SIRIPartner().NewMessageIdentifier(),
		MonitoringRef:          string(request.StopAreaId()),
		RequestorRef:           connector.SIRIPartner().RequestorRef(),
		RequestTimestamp:       connector.Clock().Now(),
		SubscriberRef:          connector.SIRIPartner().RequestorRef(),
		SubscriptionIdentifier: fmt.Sprintf("Edwig:Subscription::%v:LOC", objId.Value()),
		InitialTerminationTime: connector.Clock().Now().Add(48 * time.Hour),
	}

	logSIRIStopMonitoringSubscriptionRequest(logStashEvent, siriStopMonitoringSubscriptionRequest)
	connector.SIRIPartner().SOAPClient().StopMonitoringSubscription(siriStopMonitoringSubscriptionRequest)
}

func (connector *SIRIStopMonitoringSubscriptionCollector) SetStopAreaUpdateSubscriber(stopAreaUpdateSubscriber StopAreaUpdateSubscriber) {
	connector.stopAreaUpdateSubscriber = stopAreaUpdateSubscriber
}

func (connector *SIRIStopMonitoringSubscriptionCollector) broadcastStopAreaUpdateEvents(events map[string]*model.StopAreaUpdateEvent) {
	if connector.stopAreaUpdateSubscriber != nil {
		for _, event := range events {
			connector.stopAreaUpdateSubscriber(event)
		}
	}
}

func (connector *SIRIStopMonitoringSubscriptionCollector) HandleNotifyStopMonitoring(delivery *siri.XMLStopMonitoringResponse) {
	logStashEvent := make(audit.LogStashEvent)
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)

	logXMLStopMonitoringDelivery(logStashEvent, delivery)

	stopAreaUpdateEvents := make(map[string]*model.StopAreaUpdateEvent)

	connector.setStopVisitUpdateEvents(stopAreaUpdateEvents, delivery)
	connector.setStopVisitCancellationEvents(stopAreaUpdateEvents, delivery)

	logStopVisitUpdateEventsFromMap(logStashEvent, stopAreaUpdateEvents)

	connector.broadcastStopAreaUpdateEvents(stopAreaUpdateEvents)
}

func (connector *SIRIStopMonitoringSubscriptionCollector) setStopVisitUpdateEvents(events map[string]*model.StopAreaUpdateEvent, xmlResponse *siri.XMLStopMonitoringResponse) {
	xmlStopVisitEvents := xmlResponse.XMLMonitoredStopVisits()
	if len(xmlStopVisitEvents) == 0 {
		return
	}

	builder := newStopVisitUpdateEventBuilder(connector.partner)

	for _, xmlStopVisitEvent := range xmlStopVisitEvents {
		stopAreaObjectId := model.NewObjectID(connector.Partner().Setting("remote_objectid_kind"), xmlStopVisitEvent.StopPointRef())
		stopArea, ok := connector.Partner().Model().StopAreas().FindByObjectId(stopAreaObjectId)
		if !ok {
			logger.Log.Debugf("StopVisitUpdateEvent for unknown StopArea %v", stopAreaObjectId.Value())
			continue
		}

		stopAreaUpdateEvent, ok := events[xmlStopVisitEvent.StopPointRef()]
		if !ok {
			stopAreaUpdateEvent = model.NewStopAreaUpdateEvent(connector.NewUUID(), stopArea.Id())
			events[xmlStopVisitEvent.StopPointRef()] = stopAreaUpdateEvent
		}
		builder.buildStopVisitUpdateEvent(stopAreaUpdateEvent, xmlStopVisitEvent)
	}
}

func (connector *SIRIStopMonitoringSubscriptionCollector) setStopVisitCancellationEvents(events map[string]*model.StopAreaUpdateEvent, xmlResponse *siri.XMLStopMonitoringResponse) {
	xmlStopVisitCancellationEvents := xmlResponse.XMLMonitoredStopVisitCancellations()
	if len(xmlStopVisitCancellationEvents) == 0 {
		return
	}

	for _, xmlStopVisitCancellationEvent := range xmlStopVisitCancellationEvents {
		stopAreaUpdateEvent, ok := events[xmlStopVisitCancellationEvent.MonitoringRef()]
		if !ok {
			stopAreaUpdateEvent = model.NewStopAreaUpdateEvent(connector.NewUUID(), model.StopAreaId(xmlStopVisitCancellationEvent.ItemRef()))
			events[xmlStopVisitCancellationEvent.MonitoringRef()] = stopAreaUpdateEvent
		}
		stopVisitCancellationEvent := &model.StopVisitNotCollectedEvent{
			StopVisitObjectId: model.NewObjectID(connector.partner.Setting("remote_objectid_kind"), xmlStopVisitCancellationEvent.ItemRef()),
		}
		stopAreaUpdateEvent.StopVisitNotCollectedEvents = append(stopAreaUpdateEvent.StopVisitNotCollectedEvents, stopVisitCancellationEvent)
	}
}

func logSIRIStopMonitoringSubscriptionRequest(logStashEvent audit.LogStashEvent, request *siri.SIRIStopMonitoringSubscriptionRequest) {
	logStashEvent["Connector"] = "StopMonitoringSubscriptionRequestCollector"
	logStashEvent["messageIdentifier"] = request.MessageIdentifier
	logStashEvent["monitoringRef"] = request.MonitoringRef
	logStashEvent["requestorRef"] = request.RequestorRef
	logStashEvent["requestTimestamp"] = request.RequestTimestamp.String()
	xml, err := request.BuildXML()
	if err != nil {
		logStashEvent["requestXML"] = fmt.Sprintf("%v", err)
		return
	}
	logStashEvent["requestXML"] = xml
}

func logXMLStopMonitoringDelivery(logStashEvent audit.LogStashEvent, delivery *siri.XMLStopMonitoringResponse) {
	logStashEvent["Connector"] = "StopMonitoringSubscriptionCollector"
	logStashEvent["address"] = delivery.Address()
	logStashEvent["producerRef"] = delivery.ProducerRef()
	logStashEvent["requestMessageRef"] = delivery.RequestMessageRef()
	logStashEvent["responseMessageIdentifier"] = delivery.ResponseMessageIdentifier()
	logStashEvent["responseTimestamp"] = delivery.ResponseTimestamp().String()
	logStashEvent["responseXML"] = delivery.RawXML()
	logStashEvent["status"] = strconv.FormatBool(delivery.Status())
	if !delivery.Status() {
		logStashEvent["errorType"] = delivery.ErrorType()
		logStashEvent["errorNumber"] = strconv.Itoa(delivery.ErrorNumber())
		logStashEvent["errorText"] = delivery.ErrorText()
		logStashEvent["errorDescription"] = delivery.ErrorDescription()
	}
}

func logStopVisitUpdateEventsFromMap(logStashEvent audit.LogStashEvent, stopAreaUpdateEvents map[string]*model.StopAreaUpdateEvent) {
	var idArray []string
	var cancelledIdArray []string
	for _, stopAreaUpdateEvent := range stopAreaUpdateEvents {
		for _, stopVisitUpdateEvent := range stopAreaUpdateEvent.StopVisitUpdateEvents {
			idArray = append(idArray, stopVisitUpdateEvent.Id)
		}
		for _, stopVisitCancelledEvent := range stopAreaUpdateEvent.StopVisitNotCollectedEvents {
			cancelledIdArray = append(cancelledIdArray, stopVisitCancelledEvent.StopVisitObjectId.Value())
		}
	}
	logStashEvent["StopVisitUpdateEventIds"] = strings.Join(idArray, ", ")
	logStashEvent["StopVisitCancelledEventIds"] = strings.Join(cancelledIdArray, ", ")
}
