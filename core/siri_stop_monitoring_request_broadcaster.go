package core

import (
	"fmt"
	"time"

	"github.com/af83/edwig/audit"
	"github.com/af83/edwig/model"
	"github.com/af83/edwig/siri"
)

type StopMonitoringRequestBroadcaster interface {
	RequestStopArea(request *siri.XMLStopMonitoringRequest) (*siri.SIRIStopMonitoringResponse, error)
}

type SIRIStopMonitoringRequestBroadcaster struct {
	model.ClockConsumer

	siriConnector
}

type SIRIStopMonitoringRequestBroadcasterFactory struct{}

func NewSIRIStopMonitoringRequestBroadcaster(partner *Partner) *SIRIStopMonitoringRequestBroadcaster {
	siriStopMonitoringRequestBroadcaster := &SIRIStopMonitoringRequestBroadcaster{}
	siriStopMonitoringRequestBroadcaster.partner = partner
	return siriStopMonitoringRequestBroadcaster
}

func (connector *SIRIStopMonitoringRequestBroadcaster) RequestStopArea(request *siri.XMLStopMonitoringRequest) (*siri.SIRIStopMonitoringResponse, error) {
	tx := connector.Partner().Referential().NewTransaction()
	defer tx.Close()

	objectidKind := connector.Partner().Setting("remote_objectid_kind")
	objectid := model.NewObjectID(objectidKind, request.MonitoringRef())
	stopArea, ok := tx.Model().StopAreas().FindByObjectId(objectid)

	if !ok {
		return nil, fmt.Errorf("StopArea not found")
	}

	if stopArea.MonitoredAlways == false {
		stopArea.MonitoredUntil = connector.Clock().Now().Add(time.Duration(15) * time.Minute)
	}

	logStashEvent := make(audit.LogStashEvent)
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)

	logXMLStopMonitoringRequest(logStashEvent, request)

	response := new(siri.SIRIStopMonitoringResponse)
	response.Address = connector.Partner().Setting("local_url")
	response.ProducerRef = connector.Partner().Setting("remote_credential")
	if response.ProducerRef == "" {
		response.ProducerRef = "Edwig"
	}
	response.RequestMessageRef = request.MessageIdentifier()
	response.ResponseMessageIdentifier = connector.SIRIPartner().NewMessageIdentifier()
	response.Status = true
	response.ResponseTimestamp = connector.Clock().Now()
	// Fill StopVisits
	for _, stopVisit := range tx.Model().StopVisits().FindByStopAreaId(stopArea.Id()) {
		var itemIdentifier string
		stopVisitId, ok := stopVisit.ObjectID(objectidKind)
		if ok {
			itemIdentifier = stopVisitId.Value()
		} else {
			defaultObjectID, ok := stopVisit.ObjectID("_default")
			if !ok {
				continue
			}
			itemIdentifier = fmt.Sprintf("RATPDEV:Item::%s:LOC", defaultObjectID.HashValue())
		}

		schedules := stopVisit.Schedules
		vehicleJourney := stopVisit.VehicleJourney()
		if vehicleJourney == nil {
			continue
		}
		line := vehicleJourney.Line()
		if line == nil {
			continue
		}

		modelDate := tx.Model().Date()

		LineObjectId, _ := line.ObjectID(objectidKind)

		monitoredStopVisit := &siri.SIRIMonitoredStopVisit{
			ItemIdentifier: itemIdentifier,
			StopPointRef:   objectid.Value(),
			StopPointName:  stopArea.Name,

			VehicleJourneyName:    vehicleJourney.Name,
			LineRef:               LineObjectId.Value(),
			DataFrameRef:          fmt.Sprintf("RATPDev:DataFrame::%s:LOC", modelDate.String()),
			RecordedAt:            stopVisit.RecordedAt,
			PublishedLineName:     line.Name,
			DepartureStatus:       string(stopVisit.DepartureStatus),
			ArrivalStatus:         string(stopVisit.ArrivalStatus),
			Order:                 stopVisit.PassageOrder,
			VehicleAtStop:         stopVisit.VehicleAtStop,
			AimedArrivalTime:      schedules.Schedule(model.STOP_VISIT_SCHEDULE_AIMED).ArrivalTime(),
			ExpectedArrivalTime:   schedules.Schedule(model.STOP_VISIT_SCHEDULE_EXPECTED).ArrivalTime(),
			ActualArrivalTime:     schedules.Schedule(model.STOP_VISIT_SCHEDULE_ACTUAL).ArrivalTime(),
			AimedDepartureTime:    schedules.Schedule(model.STOP_VISIT_SCHEDULE_AIMED).DepartureTime(),
			ExpectedDepartureTime: schedules.Schedule(model.STOP_VISIT_SCHEDULE_EXPECTED).DepartureTime(),
			ActualDepartureTime:   schedules.Schedule(model.STOP_VISIT_SCHEDULE_ACTUAL).DepartureTime(),
			Attributes:            make(map[string]map[string]string),
			References:            make(map[string]map[string]model.Reference),
		}
		connector.resolveVehiculeJourneyReferences(vehicleJourney.References, tx.Model().StopAreas())
		connector.reformatVehiculeJourneyReferences(vehicleJourney.References, tx.Model().StopAreas())

		monitoredStopVisit.Attributes["StopVisitAttributes"] = stopVisit.Attributes
		monitoredStopVisit.Attributes["VehicleJourneyAttributes"] = vehicleJourney.Attributes

		monitoredStopVisit.References["VehicleJourney"] = vehicleJourney.References

		response.MonitoredStopVisits = append(response.MonitoredStopVisits, monitoredStopVisit)
	}

	logSIRIStopMonitoringResponse(logStashEvent, response)

	return response, nil
}

func (connector *SIRIStopMonitoringRequestBroadcaster) resolveVehiculeJourneyReferences(references map[string]model.Reference, manager model.StopAreas) {
	toResolve := []string{"PlaceRef", "OriginRef", "DestinationRef"}

	for _, ref := range toResolve {
		if references[ref] != (model.Reference{}) {
			if foundStopArea, ok := manager.Find(model.StopAreaId(references[ref].Id)); ok {
				obj, ok := foundStopArea.ObjectID(connector.Partner().Setting("remote_objectid_kind"))
				if ok {
					tmp := references[ref]
					tmp.ObjectId = &obj
					references[ref] = tmp
				}
			} else {
				tmp := references[ref]
				tmp.ObjectId.SetValue(tmp.Getformat(ref, tmp.GetSha1()))
			}
		}
	}
}

func (connector *SIRIStopMonitoringRequestBroadcaster) reformatVehiculeJourneyReferences(references map[string]model.Reference, manager model.StopAreas) {
	toReformat := []string{"RouteRef", "JourneyPatternRef"}

	for _, ref := range toReformat {
		if references[ref] != (model.Reference{}) {
			tmp := references[ref]
			tmp.ObjectId.SetValue(tmp.Getformat(ref, tmp.GetSha1()))
		}
	}
}

func (factory *SIRIStopMonitoringRequestBroadcasterFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	ok = ok && apiPartner.ValidatePresenceOfSetting("local_credential")
	return ok
}

func (factory *SIRIStopMonitoringRequestBroadcasterFactory) CreateConnector(partner *Partner) Connector {
	return NewSIRIStopMonitoringRequestBroadcaster(partner)
}

func logXMLStopMonitoringRequest(logStashEvent audit.LogStashEvent, request *siri.XMLStopMonitoringRequest) {
	logStashEvent["messageIdentifier"] = request.MessageIdentifier()
	logStashEvent["monitoringRef"] = request.MonitoringRef()
	logStashEvent["requestorRef"] = request.RequestorRef()
	logStashEvent["requestTimestamp"] = request.RequestTimestamp().String()
	logStashEvent["requestXML"] = request.RawXML()
}

func logSIRIStopMonitoringResponse(logStashEvent audit.LogStashEvent, response *siri.SIRIStopMonitoringResponse) {
	logStashEvent["address"] = response.Address
	logStashEvent["producerRef"] = response.ProducerRef
	logStashEvent["requestMessageRef"] = response.RequestMessageRef
	logStashEvent["responseMessageIdentifier"] = response.ResponseMessageIdentifier
	logStashEvent["responseTimestamp"] = response.ResponseTimestamp.String()
	xml, err := response.BuildXML()
	if err != nil {
		logStashEvent["responseXML"] = fmt.Sprintf("%v", err)
		return
	}
	logStashEvent["responseXML"] = xml
}
