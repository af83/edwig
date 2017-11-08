package core

import (
	"strconv"
	"strings"
	"sync"

	"github.com/af83/edwig/audit"
	"github.com/af83/edwig/model"
	"github.com/af83/edwig/siri"
)

type GeneralMessageSubscriptionBroadcaster interface {
	model.Stopable
	model.Startable

	HandleGeneralMessageBroadcastEvent(*model.GeneralMessageBroadcastEvent)
	HandleSubscriptionRequest(*siri.XMLSubscriptionRequest)
}

type SIRIGeneralMessageSubscriptionBroadcaster struct {
	model.ClockConsumer
	model.UUIDConsumer

	siriConnector

	generalMessageBroadcaster SIRIGeneralMessageBroadcaster
	toBroadcast               map[SubscriptionId][]model.SituationId
	mutex                     *sync.Mutex //protect the map
}

type SIRIGeneralMessageSubscriptionBroadcasterFactory struct{}

func (factory *SIRIGeneralMessageSubscriptionBroadcasterFactory) CreateConnector(partner *Partner) Connector {
	if _, ok := partner.Connector(SIRI_SUBSCRIPTION_REQUEST_DISPATCHER); !ok {
		partner.CreateSubscriptionRequestDispatcher()
	}
	return newSIRIGeneralMessageSubscriptionBroadcaster(partner)
}

func (factory *SIRIGeneralMessageSubscriptionBroadcasterFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	ok = ok && apiPartner.ValidatePresenceOfSetting("remote_url")
	ok = ok && apiPartner.ValidatePresenceOfSetting("remote_credential")
	return ok
}

func newSIRIGeneralMessageSubscriptionBroadcaster(partner *Partner) *SIRIGeneralMessageSubscriptionBroadcaster {
	siriGeneralMessageSubscriptionBroadcaster := &SIRIGeneralMessageSubscriptionBroadcaster{}
	siriGeneralMessageSubscriptionBroadcaster.partner = partner
	siriGeneralMessageSubscriptionBroadcaster.mutex = &sync.Mutex{}
	siriGeneralMessageSubscriptionBroadcaster.toBroadcast = make(map[SubscriptionId][]model.SituationId)

	siriGeneralMessageSubscriptionBroadcaster.generalMessageBroadcaster = NewSIRIGeneralMessageBroadcaster(siriGeneralMessageSubscriptionBroadcaster)

	return siriGeneralMessageSubscriptionBroadcaster
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) Stop() {
	if connector.generalMessageBroadcaster != nil {
		connector.generalMessageBroadcaster.Stop()
	}
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) Start() {
	if connector.generalMessageBroadcaster == nil {
		connector.generalMessageBroadcaster = NewSIRIGeneralMessageBroadcaster(connector)
	}
	connector.generalMessageBroadcaster.Start()
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) HandleGeneralMessageBroadcastEvent(event *model.GeneralMessageBroadcastEvent) {
	subId, ok := connector.checkEvent(event.SituationId)
	if ok {
		connector.addSituation(subId, event.SituationId)
	}
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) addSituation(subId SubscriptionId, svId model.SituationId) {
	connector.mutex.Lock()
	connector.toBroadcast[SubscriptionId(subId)] = append(connector.toBroadcast[SubscriptionId(subId)], svId)
	connector.mutex.Unlock()
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) checkEvent(sId model.SituationId) (SubscriptionId, bool) {
	subId := SubscriptionId(0) //just to return a correct type for errors
	tx := connector.Partner().Referential().NewTransaction()
	defer tx.Close()

	_, ok := tx.Model().Situations().Find(sId)
	if !ok {
		return subId, false
	}

	sub, ok := connector.partner.Subscriptions().FindByKind("GeneralMessageBroadcast")
	if !ok {
		return subId, false
	}

	return sub.Id(), true
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) HandleSubscriptionRequest(request *siri.XMLSubscriptionRequest) []siri.SIRIResponseStatus {
	resps := []siri.SIRIResponseStatus{}

	for _, gm := range request.XMLSubscriptionGMEntries() {
		logStashEvent := connector.newLogStashEvent()
		logXMLGeneralMessageSubscriptionEntry(logStashEvent, gm)

		rs := siri.SIRIResponseStatus{
			RequestMessageRef: gm.MessageIdentifier(),
			SubscriberRef:     gm.SubscriberRef(),
			SubscriptionRef:   gm.SubscriptionIdentifier(),
			Status:            true,
			ResponseTimestamp: connector.Clock().Now(),
			ValidUntil:        gm.InitialTerminationTime(),
		}

		sub, ok := connector.Partner().Subscriptions().FindByExternalId(gm.SubscriptionIdentifier())
		if !ok {
			sub = connector.Partner().Subscriptions().New("GeneralMessageBroadcast")
			sub.SetExternalId(gm.SubscriptionIdentifier())
		}

		sub.SubscriptionOptions()["InfoChannelRef"] = strings.Join(gm.InfoChannelRef(), ",")
		sub.SubscriptionOptions()["LineRef"] = strings.Join(gm.LineRef(), ",")
		sub.SubscriptionOptions()["StopPointRef"] = strings.Join(gm.StopPointRef(), ",")
		sub.SubscriptionOptions()["MessageIdentifier"] = gm.MessageIdentifier()
		sub.Save()

		connector.addSituations(sub.Id())
		logSIRIGeneralMessageSubscriptionResponseEntry(logStashEvent, &rs)
		audit.CurrentLogStash().WriteEvent(logStashEvent)
		resps = append(resps, rs)
	}
	return resps
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) addSituations(subId SubscriptionId) {
	for _, situation := range connector.partner.Model().Situations().FindAll() {
		connector.addSituation(subId, situation.Id())
	}
}

func (connector *SIRIGeneralMessageSubscriptionBroadcaster) newLogStashEvent() audit.LogStashEvent {
	event := connector.partner.NewLogStashEvent()
	event["connector"] = "GeneralMessageSubscriptionBroadcaster"
	return event
}

func logXMLGeneralMessageSubscriptionEntry(logStashEvent audit.LogStashEvent, request *siri.XMLGeneralMessageSubscriptionRequestEntry) {
	logStashEvent["type"] = "GeneralMessageSubscriptionEntry"
	logStashEvent["messageIdentifier"] = request.MessageIdentifier()
	logStashEvent["requestTimestamp"] = request.RequestTimestamp().String()
	logStashEvent["infoChannelRef"] = strings.Join(request.InfoChannelRef(), ", ")
	logStashEvent["groupOfLinesRef"] = strings.Join(request.GroupOfLinesRef(), ", ")
	logStashEvent["routeRef"] = strings.Join(request.RouteRef(), ", ")
	logStashEvent["destinationRef"] = strings.Join(request.DestinationRef(), ", ")
	logStashEvent["journeyPatternRef"] = strings.Join(request.JourneyPatternRef(), ", ")
	logStashEvent["stopPointRef"] = strings.Join(request.StopPointRef(), ", ")
	logStashEvent["lineRef"] = strings.Join(request.LineRef(), ", ")
	logStashEvent["subscriberRef"] = request.SubscriberRef()
	logStashEvent["subscriptionIdentifier"] = request.SubscriptionIdentifier()
	logStashEvent["initialTerminationTime"] = request.InitialTerminationTime().String()
	logStashEvent["requestXML"] = request.RawXML()
}

func logSIRIGeneralMessageSubscriptionResponseEntry(logStashEvent audit.LogStashEvent, gmEntry *siri.SIRIResponseStatus) {
	logStashEvent["requestMessageRef"] = gmEntry.RequestMessageRef
	logStashEvent["subscriptionRef"] = gmEntry.SubscriptionRef
	logStashEvent["responseTimestamp"] = gmEntry.ResponseTimestamp.String()
	logStashEvent["validUntil"] = gmEntry.ValidUntil.String()
	logStashEvent["status"] = strconv.FormatBool(gmEntry.Status)
	if !gmEntry.Status {
		logStashEvent["errorType"] = gmEntry.ErrorType
		if gmEntry.ErrorType == "OtherError" {
			logStashEvent["errorNumber"] = strconv.Itoa(gmEntry.ErrorNumber)
		}
		logStashEvent["errorText"] = gmEntry.ErrorText
	}
}

// Start Test

type TestSIRIGeneralMessageSubscriptionBroadcasterFactory struct{}

type TestGeneralMessageSubscriptionBroadcaster struct {
	model.UUIDConsumer

	events                    []*model.GeneralMessageBroadcastEvent
	generalMessageBroadcaster SIRIGeneralMessageBroadcaster
}

func NewTestGeneralMessageSubscriptionBroadcaster() *TestGeneralMessageSubscriptionBroadcaster {
	connector := &TestGeneralMessageSubscriptionBroadcaster{}
	return connector
}

func (connector *TestGeneralMessageSubscriptionBroadcaster) HandleGeneralMessageBroadcastEvent(event *model.GeneralMessageBroadcastEvent) {
	connector.events = append(connector.events, event)
}

func (factory *TestSIRIGeneralMessageSubscriptionBroadcasterFactory) Validate(apiPartner *APIPartner) bool {
	return true
}

func (factory *TestSIRIGeneralMessageSubscriptionBroadcasterFactory) CreateConnector(partner *Partner) Connector {
	return NewTestGeneralMessageSubscriptionBroadcaster()
}

// END OF TEST
