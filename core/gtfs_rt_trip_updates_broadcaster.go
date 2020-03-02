package core

import (
	"fmt"
	"sort"
	"strconv"

	"bitbucket.org/enroute-mobi/edwig/audit"
	"bitbucket.org/enroute-mobi/edwig/logger"
	"bitbucket.org/enroute-mobi/edwig/model"
	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
)

type TripUpdatesBroadcaster struct {
	model.ClockConsumer

	BaseConnector
}

type TripUpdatesBroadcasterFactory struct{}

func (factory *TripUpdatesBroadcasterFactory) CreateConnector(partner *Partner) Connector {
	return NewTripUpdatesBroadcaster(partner)
}

func (factory *TripUpdatesBroadcasterFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	return ok
}

func NewTripUpdatesBroadcaster(partner *Partner) *TripUpdatesBroadcaster {
	connector := &TripUpdatesBroadcaster{}
	connector.partner = partner

	return connector
}

func (connector *TripUpdatesBroadcaster) HandleGtfs(feed *gtfs.FeedMessage, logStashEvent audit.LogStashEvent) {
	tx := connector.Partner().Referential().NewTransaction()
	defer tx.Close()

	stopVisits := tx.Model().StopVisits().FindAll()
	linesObjectId := make(map[model.VehicleJourneyId]model.ObjectID)
	feedEntities := make(map[model.VehicleJourneyId]*gtfs.FeedEntity)

	objectidKind := connector.partner.RemoteObjectIDKind(GTFS_RT_TRIP_UPDATES_BROADCASTER)

	for i := range stopVisits {
		sa, ok := tx.Model().StopAreas().Find(stopVisits[i].StopAreaId)
		if !ok { // Should never happen
			logger.Log.Debugf("Can't find StopArea %v of StopVisit %v", stopVisits[i].StopAreaId, stopVisits[i].Id())
			continue
		}
		saId, ok := sa.ObjectID(objectidKind)
		if !ok {
			continue
		}

		feedEntity, ok := feedEntities[stopVisits[i].VehicleJourneyId]
		// If we don't already have a tripUpdate with the VehicleJourney we create one
		if !ok {
			// Fetch all needed models and objectids
			vj, ok := tx.Model().VehicleJourneys().Find(stopVisits[i].VehicleJourneyId)
			if !ok {
				continue
			}
			vjId, ok := vj.ObjectID(objectidKind)
			if !ok {
				continue
			}

			var routeId string
			lineObjectid, ok := linesObjectId[vj.Id()]
			if !ok {
				l, ok := tx.Model().Lines().Find(vj.LineId)
				if !ok {
					continue
				}
				lineObjectid, ok = l.ObjectID(objectidKind)
				if !ok {
					continue
				}
				linesObjectId[stopVisits[i].VehicleJourneyId] = lineObjectid
			}
			routeId = lineObjectid.Value()

			// Fill the tripDescriptor
			tripId := vjId.Value()
			tripDescriptor := &gtfs.TripDescriptor{
				TripId:  &tripId,
				RouteId: &routeId,
			}

			// Fill the FeedEntity
			newId := fmt.Sprintf("trip:%v", vjId.Value())
			feedEntity = &gtfs.FeedEntity{
				Id:         &newId,
				TripUpdate: &gtfs.TripUpdate{Trip: tripDescriptor},
			}

			feedEntities[stopVisits[i].VehicleJourneyId] = feedEntity
		}

		stopId := saId.Value()
		stopSequence := uint32(stopVisits[i].PassageOrder)
		arrival := &gtfs.TripUpdate_StopTimeEvent{}
		departure := &gtfs.TripUpdate_StopTimeEvent{}

		if a := stopVisits[i].ReferenceArrivalTime(); !a.IsZero() {
			arrivalTime := int64(a.Unix())
			arrival.Time = &arrivalTime
		}
		if d := stopVisits[i].ReferenceDepartureTime(); !d.IsZero() {
			departureTime := int64(d.Unix())
			departure.Time = &departureTime
		}

		stopTimeUpdate := &gtfs.TripUpdate_StopTimeUpdate{
			StopSequence: &stopSequence,
			StopId:       &stopId,
			Arrival:      arrival,
			Departure:    departure,
		}

		feedEntity.TripUpdate.StopTimeUpdate = append(feedEntity.TripUpdate.StopTimeUpdate, stopTimeUpdate)
	}

	var n int
	for _, entity := range feedEntities {
		if len(entity.TripUpdate.StopTimeUpdate) == 0 {
			continue
		}
		sort.Slice(entity.TripUpdate.StopTimeUpdate, func(i, j int) bool {
			return *entity.TripUpdate.StopTimeUpdate[i].StopSequence < *entity.TripUpdate.StopTimeUpdate[j].StopSequence
		})
		feed.Entity = append(feed.Entity, entity)
		n++
	}

	logStashEvent["trip_update_quantity"] = strconv.Itoa(n)
}
