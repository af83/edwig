package core

import (
	"strconv"
	"time"

	external_models "github.com/af83/ara-external-models"
	"github.com/af83/edwig/audit"
	"github.com/af83/edwig/logger"
	"github.com/af83/edwig/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type PushCollector struct {
	model.ClockConsumer
	model.UUIDConsumer

	BaseConnector

	subscriber UpdateSubscriber
}

type PushCollectorFactory struct{}

func (factory *PushCollectorFactory) CreateConnector(partner *Partner) Connector {
	return NewPushCollector(partner)
}

func (factory *PushCollectorFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	ok = ok && apiPartner.ValidatePresenceOfSetting(LOCAL_CREDENTIAL)
	return ok
}

func NewPushCollector(partner *Partner) *PushCollector {
	connector := &PushCollector{}
	connector.partner = partner
	manager := partner.Referential().CollectManager()
	connector.subscriber = manager.BroadcastUpdateEvent

	return connector
}

func (pc *PushCollector) SetSubscriber(subscriber UpdateSubscriber) {
	pc.subscriber = subscriber
}

func (pc *PushCollector) broadcastUpdateEvent(event model.UpdateEvent) {
	if pc.subscriber != nil {
		pc.subscriber(event)
	}
}

func (pc *PushCollector) HandlePushNotification(model *external_models.ExternalCompleteModel) {
	t := time.Now()

	pc.handleStopAreas(model.GetStopAreas())
	pc.handleLines(model.GetLines())
	pc.handleVehicleJourneys(model.GetVehicleJourneys())
	pc.handleStopVisits(model.GetStopVisits())

	total := len(model.GetStopAreas()) + len(model.GetLines()) + len(model.GetVehicleJourneys()) + len(model.GetStopVisits())
	logger.Log.Debugf("PushCollector handled %v models in %v", total, time.Since(t))

	logStashEvent := pc.newLogStashEvent()
	pc.logPushNotification(logStashEvent, model)
	audit.CurrentLogStash().WriteEvent(logStashEvent)
}

func (pc *PushCollector) handleStopAreas(sas []*external_models.ExternalStopArea) {
	partner := string(pc.Partner().Slug())
	id_kind := pc.Partner().Setting("remote_objectid_kind")

	for i := range sas {
		sa := sas[i]
		event := model.NewStopAreaUpdateEvent()

		event.Origin = partner
		event.ObjectId = model.NewObjectID(id_kind, sa.GetObjectid())
		event.Name = sa.GetName()
		event.CollectedAlways = true
		event.Longitude = sa.GetLongitude()
		event.Latitude = sa.GetLatitude()

		pc.broadcastUpdateEvent(event)
	}
}

func (pc *PushCollector) handleLines(lines []*external_models.ExternalLine) {
	partner := string(pc.Partner().Slug())
	id_kind := pc.Partner().Setting("remote_objectid_kind")

	for i := range lines {
		l := lines[i]
		event := model.NewLineUpdateEvent()

		event.Origin = partner
		event.ObjectId = model.NewObjectID(id_kind, l.GetObjectid())
		event.Name = l.GetName()

		pc.broadcastUpdateEvent(event)
	}
}

func (pc *PushCollector) handleVehicleJourneys(vjs []*external_models.ExternalVehicleJourney) {
	partner := string(pc.Partner().Slug())
	id_kind := pc.Partner().Setting("remote_objectid_kind")

	for i := range vjs {
		vj := vjs[i]
		event := model.NewVehicleJourneyUpdateEvent()

		event.Origin = partner
		event.ObjectId = model.NewObjectID(id_kind, vj.GetObjectid())
		event.LineObjectId = model.NewObjectID(id_kind, vj.GetLineRef())
		event.OriginRef = vj.GetOriginRef()
		event.OriginName = vj.GetOriginName()
		event.DestinationRef = vj.GetDestinationRef()
		event.DestinationName = vj.GetDestinationName()
		event.Direction = vj.GetDirection()

		pc.broadcastUpdateEvent(event)
	}
}

func (pc *PushCollector) handleStopVisits(svs []*external_models.ExternalStopVisit) {
	partner := string(pc.Partner().Slug())
	id_kind := pc.Partner().Setting("remote_objectid_kind")

	for i := range svs {
		sv := svs[i]
		event := model.NewStopVisitUpdateEvent()

		event.Origin = partner
		event.ObjectId = model.NewObjectID(id_kind, sv.GetObjectid())
		event.StopAreaObjectId = model.NewObjectID(id_kind, sv.GetStopAreaRef())
		event.VehicleJourneyObjectId = model.NewObjectID(id_kind, sv.GetVehicleJourneyRef())
		event.Monitored = sv.GetMonitored()

		handleSchedules(&event.Schedules, sv.GetDepartureTimes(), sv.GetArrivalTimes())

		pc.broadcastUpdateEvent(event)
	}
}

func handleSchedules(sc *model.StopVisitSchedules, protoDeparture, protoArrival *external_models.ExternalStopVisit_Times) {
	sc.SetSchedule(model.STOP_VISIT_SCHEDULE_AIMED, convertProtoTimes(protoDeparture.GetAimed()), convertProtoTimes(protoArrival.GetAimed()))
	sc.SetSchedule(model.STOP_VISIT_SCHEDULE_ACTUAL, convertProtoTimes(protoDeparture.GetActual()), convertProtoTimes(protoArrival.GetActual()))
	sc.SetSchedule(model.STOP_VISIT_SCHEDULE_EXPECTED, convertProtoTimes(protoDeparture.GetExpected()), convertProtoTimes(protoArrival.GetExpected()))
}

func convertProtoTimes(protoTime *timestamp.Timestamp) time.Time {
	t, err := ptypes.Timestamp(protoTime)
	if err != nil {
		t = time.Time{}
	}
	return t
}

func (pc *PushCollector) newLogStashEvent() audit.LogStashEvent {
	event := pc.partner.NewLogStashEvent()
	event["connector"] = "PushCollector"
	return event
}

func (pc *PushCollector) logPushNotification(logStashEvent audit.LogStashEvent, model *external_models.ExternalCompleteModel) {
	logStashEvent["type"] = "PushNotification"
	logStashEvent["stopAreas"] = strconv.Itoa(len(model.GetStopAreas()))
	logStashEvent["lines"] = strconv.Itoa(len(model.GetLines()))
}