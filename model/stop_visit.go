package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type StopVisitId string

type StopVisitAttributes struct {
	ObjectId         ObjectID
	StopAreaObjectId ObjectID

	VehicleJourneyObjectId ObjectID
	PassageOrder           int

	ArrivalStatus   StopVisitArrivalStatus
	DepartureStatus StopVisitDepartureStatus
	RecordedAt      time.Time
	Schedules       *StopVisitSchedules
	VehicleAtStop   bool

	Attributes Attributes
	References References
}

type StopVisit struct {
	ObjectIDConsumer

	model  Model
	Origin string

	id          StopVisitId
	collected   bool
	collectedAt time.Time

	StopAreaId       StopAreaId       `json:",omitempty"`
	VehicleJourneyId VehicleJourneyId `json:",omitempty"`
	Attributes       Attributes
	References       References

	ArrivalStatus   StopVisitArrivalStatus   `json:",omitempty"`
	DepartureStatus StopVisitDepartureStatus `json:",omitempty"`
	RecordedAt      time.Time
	Schedules       *StopVisitSchedules
	VehicleAtStop   bool
	PassageOrder    int `json:",omitempty"`
}

func NewStopVisit(model Model) *StopVisit {
	stopVisit := &StopVisit{
		model:      model,
		Schedules:  NewStopVisitSchedules(),
		Attributes: NewAttributes(),
		References: NewReferences(),
	}
	stopVisit.objectids = make(ObjectIDs)
	return stopVisit
}

func (stopVisit *StopVisit) IsCollected() bool {
	return stopVisit.collected
}

func (stopVisit *StopVisit) NotCollected() {
	stopVisit.collected = false
}

func (stopVisit *StopVisit) CollectedAt() time.Time {
	return stopVisit.collectedAt
}

func (stopVisit *StopVisit) Collected(t time.Time) {
	stopVisit.collected = true
	stopVisit.collectedAt = t
}

func (stopVisit *StopVisit) Id() StopVisitId {
	return stopVisit.id
}

func (stopVisit *StopVisit) StopArea() StopArea {
	stopArea, _ := stopVisit.model.StopAreas().Find(stopVisit.StopAreaId)
	return stopArea
}

func (stopVisit *StopVisit) VehicleJourney() *VehicleJourney {
	vehicleJourney, ok := stopVisit.model.VehicleJourneys().Find(stopVisit.VehicleJourneyId)
	if !ok {
		return nil
	}
	return &vehicleJourney
}

func (stopVisit *StopVisit) MarshalJSON() ([]byte, error) {
	type Alias StopVisit
	aux := struct {
		Id          StopVisitId
		ObjectIDs   ObjectIDs `json:",omitempty"`
		Collected   bool
		CollectedAt *time.Time           `json:",omitempty"`
		RecordedAt  *time.Time           `json:",omitempty"`
		Attributes  Attributes           `json:",omitempty"`
		References  map[string]Reference `json:",omitempty"`
		Schedules   []StopVisitSchedule  `json:",omitempty"`
		*Alias
	}{
		Id:        stopVisit.id,
		Collected: stopVisit.collected,
		Alias:     (*Alias)(stopVisit),
	}

	if !stopVisit.ObjectIDs().Empty() {
		aux.ObjectIDs = stopVisit.ObjectIDs()
	}
	if !stopVisit.Attributes.IsEmpty() {
		aux.Attributes = stopVisit.Attributes
	}
	if !stopVisit.References.IsEmpty() {
		aux.References = stopVisit.References.GetReferences()
	}
	if !stopVisit.RecordedAt.IsZero() {
		aux.RecordedAt = &stopVisit.RecordedAt
	}
	if !stopVisit.collectedAt.IsZero() {
		aux.CollectedAt = &stopVisit.collectedAt
	}

	scheduleSlice := stopVisit.Schedules.ToSlice()
	if len(scheduleSlice) != 0 {
		aux.Schedules = scheduleSlice
	}

	return json.Marshal(&aux)
}

func (stopVisit *StopVisit) UnmarshalJSON(data []byte) error {
	type Alias StopVisit
	aux := &struct {
		ObjectIDs   map[string]string
		References  map[string]Reference
		CollectedAt time.Time
		Schedules   []StopVisitSchedule
		*Alias
	}{
		Alias: (*Alias)(stopVisit),
	}

	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	if aux.ObjectIDs != nil {
		stopVisit.ObjectIDConsumer.objectids = NewObjectIDsFromMap(aux.ObjectIDs)
	}

	if aux.References != nil {
		stopVisit.References.SetReferences(aux.References)
	}

	if aux.Schedules != nil {
		stopVisit.Schedules = NewStopVisitSchedules()
		for _, schedule := range aux.Schedules {
			stopVisit.Schedules.SetSchedule(schedule.Kind(), schedule.DepartureTime(), schedule.ArrivalTime())
		}
	}

	if !aux.CollectedAt.IsZero() {
		stopVisit.Collected(aux.CollectedAt)
	}
	return nil
}

func (stopVisit *StopVisit) Attribute(key string) (string, bool) {
	value, present := stopVisit.Attributes[key]
	return value, present
}

func (stopVisit *StopVisit) Save() (ok bool) {
	ok = stopVisit.model.StopVisits().Save(stopVisit)
	return
}

func (stopVisit *StopVisit) Reference(key string) (Reference, bool) {
	value, present := stopVisit.References.Get(key)
	return value, present
}

func (stopVisit *StopVisit) ReferenceTime() time.Time {
	orderMap := []StopVisitScheduleType{"actual", "expected", "aimed"}

	for _, kind := range orderMap {
		if schedule := stopVisit.Schedules.Schedule(kind); !schedule.ArrivalTime().IsZero() {
			return schedule.ArrivalTime()
		}
	}

	for _, kind := range orderMap {
		if schedule := stopVisit.Schedules.Schedule(kind); !schedule.DepartureTime().IsZero() {
			return schedule.DepartureTime()
		}
	}

	return time.Time{}
}

type ByTime []StopVisit

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTime) Less(i, j int) bool { return !a[i].ReferenceTime().After(a[j].ReferenceTime()) }

type MemoryStopVisits struct {
	UUIDConsumer
	ClockConsumer

	model Model

	mutex          *sync.RWMutex
	byIdentifier   map[StopVisitId]*StopVisit
	broadcastEvent func(event StopMonitoringBroadcastEvent)
}

type StopVisits interface {
	UUIDInterface

	New() StopVisit
	Find(id StopVisitId) (StopVisit, bool)
	FindByObjectId(objectid ObjectID) (StopVisit, bool)
	FindByVehicleJourneyId(id VehicleJourneyId) []StopVisit
	FindFollowingByVehicleJourneyId(id VehicleJourneyId) []StopVisit
	FindByStopAreaId(id StopAreaId) []StopVisit
	FindFollowingByStopAreaId(id StopAreaId) []StopVisit
	FindFollowingByStopAreaIds(stopAreaIds []StopAreaId) []StopVisit
	FindAll() []StopVisit
	Save(stopVisit *StopVisit) bool
	Delete(stopVisit *StopVisit) bool
}

func NewMemoryStopVisits() *MemoryStopVisits {
	return &MemoryStopVisits{
		mutex:        &sync.RWMutex{},
		byIdentifier: make(map[StopVisitId]*StopVisit),
	}
}

func (manager *MemoryStopVisits) New() StopVisit {
	stopVisit := NewStopVisit(manager.model)
	return *stopVisit
}

func (manager *MemoryStopVisits) Find(id StopVisitId) (StopVisit, bool) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	stopVisit, ok := manager.byIdentifier[id]

	if ok {
		return *stopVisit, true
	} else {
		return StopVisit{}, false
	}
}

func (manager *MemoryStopVisits) FindByObjectId(objectid ObjectID) (StopVisit, bool) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for _, stopVisit := range manager.byIdentifier {
		stopVisitObjectId, _ := stopVisit.ObjectID(objectid.Kind())
		if stopVisitObjectId.Value() == objectid.Value() {
			return *stopVisit, true
		}
	}
	return StopVisit{}, false
}

func (manager *MemoryStopVisits) FindByVehicleJourneyId(id VehicleJourneyId) (stopVisits []StopVisit) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for _, stopVisit := range manager.byIdentifier {
		if stopVisit.VehicleJourneyId == id {
			stopVisits = append(stopVisits, *stopVisit)
		}
	}
	return
}

func (manager *MemoryStopVisits) FindFollowingByVehicleJourneyId(id VehicleJourneyId) (stopVisits []StopVisit) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for _, stopVisit := range manager.byIdentifier {
		if stopVisit.VehicleJourneyId == id && stopVisit.ReferenceTime().After(manager.Clock().Now()) {
			stopVisits = append(stopVisits, *stopVisit)
		}
	}
	sort.Sort(ByTime(stopVisits))
	return
}

func (manager *MemoryStopVisits) FindByStopAreaId(id StopAreaId) (stopVisits []StopVisit) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for _, stopVisit := range manager.byIdentifier {
		if stopVisit.StopAreaId == id {
			stopVisits = append(stopVisits, *stopVisit)
		}
	}
	return
}

func (manager *MemoryStopVisits) FindFollowingByStopAreaId(id StopAreaId) (stopVisits []StopVisit) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for _, stopVisit := range manager.byIdentifier {
		if stopVisit.StopAreaId == id && stopVisit.ReferenceTime().After(manager.Clock().Now()) {
			stopVisits = append(stopVisits, *stopVisit)
		}
	}
	sort.Sort(ByTime(stopVisits))
	return
}

func (manager *MemoryStopVisits) FindFollowingByStopAreaIds(stopAreaIds []StopAreaId) (stopVisits []StopVisit) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for _, stopAreaId := range stopAreaIds {
		stopVisits = append(stopVisits, manager.FindFollowingByStopAreaId(stopAreaId)...)
	}
	sort.Sort(ByTime(stopVisits))
	return
}

func (manager *MemoryStopVisits) FindAll() (stopVisits []StopVisit) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	if len(manager.byIdentifier) == 0 {
		return []StopVisit{}
	}
	for _, stopVisit := range manager.byIdentifier {
		stopVisits = append(stopVisits, *stopVisit)
	}
	return
}

func (manager *MemoryStopVisits) Save(stopVisit *StopVisit) bool {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if stopVisit.id == "" {
		stopVisit.id = StopVisitId(manager.NewUUID())
	}

	stopVisit.model = manager.model
	manager.byIdentifier[stopVisit.id] = stopVisit

	event := StopMonitoringBroadcastEvent{
		ModelId:   string(stopVisit.id),
		ModelType: "StopVisit",
	}

	if manager.broadcastEvent != nil {
		manager.broadcastEvent(event)
	}

	return true
}

func (manager *MemoryStopVisits) Delete(stopVisit *StopVisit) bool {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	delete(manager.byIdentifier, stopVisit.Id())
	return true
}

func (manager *MemoryStopVisits) Load(referentialSlug string) error {
	var selectStopVisits []SelectStopVisit
	modelName := manager.model.Date()

	sqlQuery := fmt.Sprintf("select * from stop_visits where referential_slug = '%s' and model_name = '%s'", referentialSlug, modelName.String())
	_, err := Database.Select(&selectStopVisits, sqlQuery)
	if err != nil {
		return err
	}
	for _, sv := range selectStopVisits {
		stopVisit := manager.New()
		stopVisit.id = StopVisitId(sv.Id)
		if sv.StopAreaId.Valid {
			stopVisit.StopAreaId = StopAreaId(sv.StopAreaId.String)
		}
		if sv.VehicleJourneyId.Valid {
			stopVisit.VehicleJourneyId = VehicleJourneyId(sv.VehicleJourneyId.String)
		}
		if sv.PassageOrder.Valid {
			stopVisit.PassageOrder = int(sv.PassageOrder.Int64)
		}

		if sv.Attributes.Valid && len(sv.Attributes.String) > 0 {
			if err = json.Unmarshal([]byte(sv.Attributes.String), &stopVisit.Attributes); err != nil {
				return err
			}
		}

		if sv.References.Valid && len(sv.References.String) > 0 {
			if err = json.Unmarshal([]byte(sv.References.String), &stopVisit.References); err != nil {
				return err
			}
		}

		if sv.ObjectIDs.Valid && len(sv.ObjectIDs.String) > 0 {
			objectIdMap := make(map[string]string)
			if err = json.Unmarshal([]byte(sv.ObjectIDs.String), &objectIdMap); err != nil {
				return err
			}
			stopVisit.objectids = NewObjectIDsFromMap(objectIdMap)
		}

		if sv.Schedules.Valid && len(sv.Schedules.String) > 0 {
			scheduleSlice := []StopVisitSchedule{}
			if err = json.Unmarshal([]byte(sv.Schedules.String), &scheduleSlice); err != nil {
				return err
			}
			stopVisit.Schedules = NewStopVisitSchedules()
			for _, schedule := range scheduleSlice {
				stopVisit.Schedules.SetSchedule(schedule.Kind(), schedule.DepartureTime(), schedule.ArrivalTime())
			}
		}

		manager.Save(&stopVisit)
	}
	return nil
}
