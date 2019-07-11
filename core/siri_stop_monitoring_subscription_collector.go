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
	model.Stopable
	model.Startable

	RequestStopAreaUpdate(request *StopAreaUpdateRequest)
	HandleNotifyStopMonitoring(delivery *siri.XMLNotifyStopMonitoring)
	HandleTerminatedNotification(termination *siri.XMLStopMonitoringSubscriptionTerminatedResponse)
}

type SIRIStopMonitoringSubscriptionCollector struct {
	model.ClockConsumer
	model.UUIDConsumer

	siriConnector

	stopMonitoringSubscriber SIRIStopMonitoringSubscriber
	stopAreaUpdateSubscriber StopAreaUpdateSubscriber
}

type SIRIStopMonitoringSubscriptionCollectorFactory struct{}

func (factory *SIRIStopMonitoringSubscriptionCollectorFactory) CreateConnector(partner *Partner) Connector {
	return NewSIRIStopMonitoringSubscriptionCollector(partner)
}

func (factory *SIRIStopMonitoringSubscriptionCollectorFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	ok = ok && apiPartner.ValidatePresenceOfSetting("remote_url")
	ok = ok && apiPartner.ValidatePresenceOfSetting("remote_credential")
	ok = ok && apiPartner.ValidatePresenceOfSetting("local_credential")
	return ok
}

func NewSIRIStopMonitoringSubscriptionCollector(partner *Partner) *SIRIStopMonitoringSubscriptionCollector {
	connector := &SIRIStopMonitoringSubscriptionCollector{}
	connector.partner = partner
	manager := partner.Referential().CollectManager()
	connector.stopAreaUpdateSubscriber = manager.BroadcastLegacyStopAreaUpdateEvent
	connector.stopMonitoringSubscriber = NewSIRIStopMonitoringSubscriber(connector)

	return connector
}

func (connector *SIRIStopMonitoringSubscriptionCollector) Stop() {
	connector.stopMonitoringSubscriber.Stop()
}

func (connector *SIRIStopMonitoringSubscriptionCollector) Start() {
	connector.stopMonitoringSubscriber.Start()
}

func (connector *SIRIStopMonitoringSubscriptionCollector) RequestStopAreaUpdate(request *StopAreaUpdateRequest) {
	tx := connector.Partner().Referential().NewTransaction()
	defer tx.Close()

	stopArea, ok := tx.Model().StopAreas().Find(request.StopAreaId())
	if !ok {
		logger.Log.Debugf("StopAreaUpdateRequest in StopMonitoring SubscriptionCollector for unknown StopArea %v", request.StopAreaId())
		return
	}

	objectidKind := connector.Partner().Setting("remote_objectid_kind")
	stopAreaObjectid, ok := stopArea.ObjectID(objectidKind)
	if !ok {
		logger.Log.Debugf("Requested stopArea %v doesn't have and objectId of kind %v", request.StopAreaId(), objectidKind)
		return
	}

	// Try to find a Subscription with the resource
	subscriptions := connector.partner.Subscriptions().FindByResourceId(stopAreaObjectid.String(), "StopMonitoringCollect")
	if len(subscriptions) > 0 {
		for _, subscription := range subscriptions {
			resource := subscription.Resource(stopAreaObjectid)
			if resource == nil { // Should never happen
				logger.Log.Debugf("Can't find resource in subscription after Subscriptions#FindByResourceId")
				return
			}
			if !resource.SubscribedAt.IsZero() {
				resource.SubscribedUntil = resource.SubscribedUntil.Add(2 * time.Minute)
			}
		}
		return
	}

	// Else we find or create a subscription to add the resource
	newSubscription := connector.partner.Subscriptions().FindOrCreateByKind("StopMonitoringCollect")
	ref := model.Reference{
		ObjectId: &stopAreaObjectid,
		Type:     "StopArea",
	}

	newSubscription.CreateAddNewResource(ref)
}

func (connector *SIRIStopMonitoringSubscriptionCollector) SetStopMonitoringSubscriber(stopMonitoringSubscriber SIRIStopMonitoringSubscriber) {
	connector.stopMonitoringSubscriber = stopMonitoringSubscriber
}

func (connector *SIRIStopMonitoringSubscriptionCollector) SetStopAreaUpdateSubscriber(stopAreaUpdateSubscriber StopAreaUpdateSubscriber) {
	connector.stopAreaUpdateSubscriber = stopAreaUpdateSubscriber
}

func (connector *SIRIStopMonitoringSubscriptionCollector) broadcastLegacyStopAreaUpdateEvent(event *model.LegacyStopAreaUpdateEvent) {
	if connector.stopAreaUpdateSubscriber != nil {
		connector.stopAreaUpdateSubscriber(event)
	}
}

func (connector *SIRIStopMonitoringSubscriptionCollector) HandleTerminatedNotification(response *siri.XMLStopMonitoringSubscriptionTerminatedResponse) {
	logStashEvent := make(audit.LogStashEvent)
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)

	logXMLSubscriptionTerminatedNotification(logStashEvent, response)

	connector.partner.Subscriptions().DeleteById(SubscriptionId(response.SubscriptionRef()))
}

func (connector *SIRIStopMonitoringSubscriptionCollector) HandleNotifyStopMonitoring(notify *siri.XMLNotifyStopMonitoring) {
	logStashEvent := connector.newLogStashEvent()
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)
	monitoringRefMap := make(map[string]struct{})
	subscriptionErrors := make(map[string]string)
	subToDelete := make(map[string]struct{})

	logXMLNotifyStopMonitoring(logStashEvent, notify)

	stopAreaUpdateEvents := make(map[string]*model.LegacyStopAreaUpdateEvent)
	for _, delivery := range notify.StopMonitoringDeliveries() {
		if connector.Partner().LogSubscriptionStopMonitoringDeliveries() {
			deliveryLogStashEvent := connector.newLogStashEvent()
			logXMLSubscriptionStopMonitoringDelivery(deliveryLogStashEvent, notify.ResponseMessageIdentifier(), delivery)
			audit.CurrentLogStash().WriteEvent(deliveryLogStashEvent)
		}

		subscriptionId := delivery.SubscriptionRef()

		subscription, ok := connector.Partner().Subscriptions().Find(SubscriptionId(subscriptionId))

		if !ok {
			logger.Log.Debugf("Partner %s sent a StopVisitNotify response to a non existant subscription of id: %s\n", connector.Partner().Slug(), subscriptionId)
			subscriptionErrors[subscriptionId] = "Non existant subscription of id %s"
			subToDelete[delivery.SubscriptionRef()] = struct{}{}
			continue
		}
		if subscription.Kind() != "StopMonitoringCollect" {
			logger.Log.Debugf("Partner %s sent a StopVisitNotify response to a subscription with kind: %s\n", connector.Partner().Slug(), subscription.Kind())
			subscriptionErrors[subscriptionId] = "Subscription of id %s is not a subscription of kind StopMonitoringCollect"
			continue
		}

		originStopAreaObjectId := model.ObjectID{}
		resource := subscription.UniqueResource()
		if resource != nil {
			originStopAreaObjectId = *resource.Reference.ObjectId
		} else if delivery.MonitoringRef() != "" {
			originStopAreaObjectId = model.NewObjectID(connector.Partner().Setting("remote_objectid_kind"), delivery.MonitoringRef())
		}

		tx := connector.Partner().Referential().NewTransaction()

		connector.setStopVisitUpdateEvents(stopAreaUpdateEvents, delivery, tx, monitoringRefMap, originStopAreaObjectId)
		connector.setStopVisitCancellationEvents(stopAreaUpdateEvents, delivery, tx, monitoringRefMap)

		tx.Close()
	}

	logMonitoringRefsFromMap(logStashEvent, monitoringRefMap)
	if len(subscriptionErrors) != 0 {
		logSubscriptionErrorsFromMap(logStashEvent, subscriptionErrors)
	}

	for subId := range subToDelete {
		connector.cancelSubscription(subId)
	}

	for _, event := range stopAreaUpdateEvents {
		connector.broadcastLegacyStopAreaUpdateEvent(event)
	}
}

func (connector *SIRIStopMonitoringSubscriptionCollector) cancelSubscription(subId string) {
	logStashEvent := connector.newLogStashEvent()
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)

	request := &siri.SIRIDeleteSubscriptionRequest{
		RequestTimestamp:  connector.Clock().Now(),
		SubscriptionRef:   subId,
		RequestorRef:      connector.partner.ProducerRef(),
		MessageIdentifier: connector.SIRIPartner().IdentifierGenerator("message_identifier").NewMessageIdentifier(),
	}
	logSIRIDeleteSubscriptionRequest(logStashEvent, request, "StopMonitoringSubscriptionCollector")

	startTime := connector.Clock().Now()
	response, err := connector.SIRIPartner().SOAPClient().DeleteSubscription(request)
	logStashEvent["responseTime"] = connector.Clock().Since(startTime).String()
	if err != nil {
		logger.Log.Debugf("Error while terminating subcription with id : %v error : %v", subId, err.Error())
		logStashEvent["status"] = "false"
		logStashEvent["errorDescription"] = fmt.Sprintf("Error during DeleteSubscription: %v", err)
		return
	}
	logXMLDeleteSubscriptionResponse(logStashEvent, response)
}

func (connector *SIRIStopMonitoringSubscriptionCollector) setStopVisitUpdateEvents(events map[string]*model.LegacyStopAreaUpdateEvent, xmlResponse *siri.XMLNotifyStopMonitoringDelivery, tx *model.Transaction, monitoringRefMap map[string]struct{}, originStopAreaObjectId model.ObjectID) {
	xmlStopVisitEvents := xmlResponse.XMLMonitoredStopVisits()
	if len(xmlStopVisitEvents) == 0 {
		return
	}

	builder := newStopVisitUpdateEventBuilder(connector.partner, originStopAreaObjectId)
	builder.setStopVisitUpdateEvents(events, xmlStopVisitEvents)

	for _, update := range events {
		monitoringRefMap[update.StopAreaAttributes.ObjectId.Value()] = struct{}{}
		update.SetId(connector.NewUUID())
		sa, _ := tx.Model().StopAreas().FindByObjectId(update.StopAreaAttributes.ObjectId)
		update.StopAreaId = sa.Id()
	}
}

func (connector *SIRIStopMonitoringSubscriptionCollector) setStopVisitCancellationEvents(events map[string]*model.LegacyStopAreaUpdateEvent, xmlResponse *siri.XMLNotifyStopMonitoringDelivery, tx *model.Transaction, monitoringRefMap map[string]struct{}) {
	xmlStopVisitCancellationEvents := xmlResponse.XMLMonitoredStopVisitCancellations()
	if len(xmlStopVisitCancellationEvents) == 0 {
		return
	}

	for _, xmlStopVisitCancellationEvent := range xmlStopVisitCancellationEvents {
		monitoringRefMap[xmlStopVisitCancellationEvent.MonitoringRef()] = struct{}{}

		stopAreaObjectId := model.NewObjectID(connector.Partner().Setting("remote_objectid_kind"), xmlStopVisitCancellationEvent.MonitoringRef())
		stopArea, ok := tx.Model().StopAreas().FindByObjectId(stopAreaObjectId)
		if !ok {
			logger.Log.Debugf("StopVisitCancellationEvent for unknown StopArea %v", stopAreaObjectId.Value())
			continue
		}

		stopAreaUpdateEvent, ok := events[xmlStopVisitCancellationEvent.MonitoringRef()]
		if !ok {
			stopAreaUpdateEvent = model.NewLegacyStopAreaUpdateEvent(connector.NewUUID(), stopArea.Id())
			events[xmlStopVisitCancellationEvent.MonitoringRef()] = stopAreaUpdateEvent
		}
		stopVisitCancellationEvent := &model.StopVisitNotCollectedEvent{
			StopVisitObjectId: model.NewObjectID(connector.partner.Setting("remote_objectid_kind"), xmlStopVisitCancellationEvent.ItemRef()),
		}
		stopAreaUpdateEvent.StopVisitNotCollectedEvents = append(stopAreaUpdateEvent.StopVisitNotCollectedEvents, stopVisitCancellationEvent)
	}
}

func (connector *SIRIStopMonitoringSubscriptionCollector) newLogStashEvent() audit.LogStashEvent {
	event := connector.partner.NewLogStashEvent()
	event["connector"] = "StopMonitoringSubscriptionCollector"
	return event
}

func logSIRIDeleteSubscriptionRequest(logStashEvent audit.LogStashEvent, request *siri.SIRIDeleteSubscriptionRequest, subType string) {
	logStashEvent["siriType"] = "DeleteSubscriptionRequest" // This function is also used on GM delete subscription
	logStashEvent["connector"] = subType
	logStashEvent["requestTimestamp"] = request.RequestTimestamp.String()
	logStashEvent["subscriptionRef"] = request.SubscriptionRef
	logStashEvent["requestorRef"] = request.RequestorRef
	logStashEvent["messageIdentifier"] = request.MessageIdentifier

	xml, err := request.BuildXML()
	if err != nil {
		logStashEvent["requestXML"] = fmt.Sprintf("%v", err)
		return
	}
	logStashEvent["requestXML"] = xml
}

func logXMLNotifyStopMonitoring(logStashEvent audit.LogStashEvent, notify *siri.XMLNotifyStopMonitoring) {
	logStashEvent["siriType"] = "CollectedNotifyStopMonitoring"
	logStashEvent["producerRef"] = notify.ProducerRef()
	logStashEvent["requestMessageRef"] = notify.RequestMessageRef()
	logStashEvent["responseMessageIdentifier"] = notify.ResponseMessageIdentifier()
	logStashEvent["responseTimestamp"] = notify.ResponseTimestamp().String()
	logStashEvent["responseXML"] = notify.RawXML()

	status := "true"
	errorCount := 0
	for _, delivery := range notify.StopMonitoringDeliveries() {
		if !delivery.Status() {
			status = "false"
			errorCount++
		}
	}
	logStashEvent["status"] = status
	logStashEvent["errorCount"] = strconv.Itoa(errorCount)
}

func logXMLSubscriptionStopMonitoringDelivery(logStashEvent audit.LogStashEvent, parent string, delivery *siri.XMLNotifyStopMonitoringDelivery) {
	logStashEvent["siriType"] = "CollectedNotifyStopMonitoringDelivery"
	logStashEvent["parentMessageIdentifier"] = parent
	logStashEvent["monitoringRef"] = delivery.MonitoringRef()
	logStashEvent["requestMessageRef"] = delivery.RequestMessageRef()
	logStashEvent["subscriberRef"] = delivery.SubscriberRef()
	logStashEvent["subscriptionRef"] = delivery.SubscriptionRef()
	logStashEvent["responseTimestamp"] = delivery.ResponseTimestamp().String()

	logStashEvent["status"] = strconv.FormatBool(delivery.Status())
	if !delivery.Status() {
		logStashEvent["errorType"] = delivery.ErrorType()
		if delivery.ErrorType() == "OtherError" {
			logStashEvent["errorNumber"] = strconv.Itoa(delivery.ErrorNumber())
		}
		logStashEvent["errorText"] = delivery.ErrorText()
		logStashEvent["errorDescription"] = delivery.ErrorDescription()
	}
}

func logXMLDeleteSubscriptionResponse(logStashEvent audit.LogStashEvent, response *siri.XMLDeleteSubscriptionResponse) {
	logStashEvent["responderRef"] = response.ResponderRef()
	logStashEvent["requestMessageRef"] = response.RequestMessageRef()
	logStashEvent["responseTimestamp"] = response.ResponseTimestamp().String()
	logStashEvent["responseXML"] = response.RawXML()

	var subscriptionIds []string
	for _, responseStatus := range response.ResponseStatus() {
		subscriptionIds = append(subscriptionIds, responseStatus.SubscriptionRef())
	}
	logStashEvent["subscriptionRefs"] = strings.Join(subscriptionIds, ", ")
}

func logXMLSubscriptionTerminatedNotification(logStashEvent audit.LogStashEvent, response *siri.XMLStopMonitoringSubscriptionTerminatedResponse) {
	logStashEvent["siriType"] = "TerminatedSubscriptionNotification"
	logStashEvent["producerRef"] = response.ProducerRef()
	logStashEvent["responseTimestamp"] = response.ResponseTimestamp().String()
	logStashEvent["subscriberRef"] = response.SubscriberRef()
	logStashEvent["subscriptionRef"] = response.SubscriptionRef()
	logStashEvent["responseXML"] = response.RawXML()

	logStashEvent["errorType"] = response.ErrorType()
	if response.ErrorType() == "OtherError" {
		logStashEvent["errorNumber"] = strconv.Itoa(response.ErrorNumber())
	}
	logStashEvent["errorDescription"] = response.ErrorDescription()
}

func logMonitoringRefsFromMap(logStashEvent audit.LogStashEvent, refs map[string]struct{}) {
	refSlice := make([]string, len(refs))
	i := 0
	for monitoringRef := range refs {
		refSlice[i] = monitoringRef
		i++
	}
	logStashEvent["monitoringRefs"] = strings.Join(refSlice, ", ")
}

func logSubscriptionErrorsFromMap(logStashEvent audit.LogStashEvent, errors map[string]string) {
	errSlice := make([]string, len(errors))
	i := 0
	for subId, err := range errors {
		errSlice[i] = fmt.Sprintf(err, subId)
		i++
	}
	logStashEvent["notificationErrors"] = strings.Join(errSlice, ", ")
}
