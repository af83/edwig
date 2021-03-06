package model

import "bitbucket.org/enroute-mobi/ara/uuid"

type TransactionalSituations struct {
	uuid.UUIDConsumer

	model   Model
	saved   map[SituationId]*Situation
	deleted map[SituationId]*Situation
}

func NewTransactionalSituations(model Model) *TransactionalSituations {
	situations := TransactionalSituations{model: model}
	situations.resetCaches()
	return &situations
}

func (manager *TransactionalSituations) resetCaches() {
	manager.saved = make(map[SituationId]*Situation)
	manager.deleted = make(map[SituationId]*Situation)
}

func (manager *TransactionalSituations) New() Situation {
	return *NewSituation(manager.model)
}

func (manager *TransactionalSituations) Find(id SituationId) (Situation, bool) {
	situation, ok := manager.saved[id]
	if ok {
		return *situation, ok
	}

	return manager.model.Situations().Find(id)
}

func (manager *TransactionalSituations) FindByObjectId(objectid ObjectID) (Situation, bool) {
	for _, situation := range manager.saved {
		situationObjectId, _ := situation.ObjectID(objectid.Kind())
		if situationObjectId.Value() == objectid.Value() {
			return *situation, true
		}
	}
	return manager.model.Situations().FindByObjectId(objectid)
}

func (manager *TransactionalSituations) FindAll() []Situation {
	situations := []Situation{}
	for _, situation := range manager.saved {
		situations = append(situations, *situation)
	}
	savedSituations := manager.model.Situations().FindAll()
	for _, situation := range savedSituations {
		_, ok := manager.saved[situation.Id()]
		if !ok {
			situations = append(situations, situation)
		}
	}
	return situations
}

func (manager *TransactionalSituations) Save(situation *Situation) bool {
	if situation.Id() == "" {
		situation.id = SituationId(manager.NewUUID())
	}
	manager.saved[situation.Id()] = situation
	return true
}

func (manager *TransactionalSituations) Delete(situation *Situation) bool {
	manager.deleted[situation.Id()] = situation
	return true
}

func (manager *TransactionalSituations) Commit() error {
	for _, situation := range manager.deleted {
		manager.model.Situations().Delete(situation)
	}
	for _, situation := range manager.saved {
		manager.model.Situations().Save(situation)
	}
	return nil
}

func (manager *TransactionalSituations) Rollback() error {
	manager.resetCaches()
	return nil
}
