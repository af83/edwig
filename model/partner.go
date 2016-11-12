package model

import "encoding/json"

type OperationnalStatus int

const (
	UNKNOWN OperationnalStatus = iota
	UP
	DOWN
)

type PartnerId string

type Partners interface {
	New(name string) Partner
	Find(id PartnerId) (Partner, bool)
	FindByName(name string) (Partner, bool)
	FindAll() []Partner
	Save(partner *Partner) bool
	Delete(partner *Partner) bool
}

type Partner struct {
	id                 PartnerId
	name               string
	operationnalStatus OperationnalStatus

	checkStatusClient CheckStatusClient

	manager Partners
}

type PartnerManager struct {
	UUIDConsumer

	byId map[PartnerId]*Partner
}

func (partner *Partner) Id() PartnerId {
	return partner.id
}

func (partner *Partner) Name() string {
	return partner.name
}

func (partner *Partner) OperationnalStatus() OperationnalStatus {
	return partner.operationnalStatus
}

func (partner *Partner) Save() (ok bool) {
	ok = partner.manager.Save(partner)
	return
}

func (partner *Partner) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"Id":   partner.id,
		"Name": partner.name,
	})
}

// Refresh Connector instances according to connector type list
func (partner *Partner) RefreshConnectors() {
	// WIP
	if partner.checkStatusClient != nil {
		siriPartner := NewSIRIPartner(partner)
		partner.checkStatusClient = NewSIRICheckStatusClient(siriPartner)
	}
}

func (partner *Partner) CheckStatusClient() CheckStatusClient {
	// WIP
	return partner.checkStatusClient
}

func (partner *Partner) CheckStatus() {
	partner.operationnalStatus, _ = partner.CheckStatusClient().Status()
}

func NewPartnerManager() *PartnerManager {
	return &PartnerManager{
		byId: make(map[PartnerId]*Partner),
	}
}

func (manager *PartnerManager) New(name string) Partner {
	return Partner{name: name, manager: manager}
}

func (manager *PartnerManager) Find(id PartnerId) (Partner, bool) {
	partner, ok := manager.byId[id]
	if ok {
		return *partner, true
	}
	return Partner{}, false
}

func (manager *PartnerManager) FindByName(name string) (Partner, bool) {
	for _, partner := range manager.byId {
		if partner.name == name {
			return *partner, true
		}
	}
	return Partner{}, false
}

func (manager *PartnerManager) FindAll() (partners []Partner) {
	for _, partner := range manager.byId {
		partners = append(partners, *partner)
	}
	return
}

func (manager *PartnerManager) Save(partner *Partner) bool {
	if partner.id == "" {
		partner.id = PartnerId(manager.NewUUID())
	}
	partner.manager = manager
	manager.byId[partner.id] = partner
	return true
}

func (manager *PartnerManager) Delete(partner *Partner) bool {
	delete(manager.byId, partner.id)
	return true
}
