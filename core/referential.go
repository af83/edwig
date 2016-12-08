package core

import (
	"encoding/json"

	"github.com/af83/edwig/logger"
	"github.com/af83/edwig/model"
)

type ReferentialId string
type ReferentialSlug string

type Referential struct {
	id   ReferentialId
	slug ReferentialSlug

	collectManager CollectManagerInterface
	manager        Referentials
	model          model.Model
	modelGuardian  *ModelGuardian
	partners       Partners
}

type Referentials interface {
	New(slug ReferentialSlug) *Referential
	Find(id ReferentialId) *Referential
	FindBySlug(slug ReferentialSlug) *Referential
	FindAll() []*Referential
	Save(stopArea *Referential) bool
	Delete(stopArea *Referential) bool
	Load() error
}

var referentials = NewMemoryReferentials()

type APIReferential struct {
	Id     ReferentialId `json:"Id,omitempty"`
	Slug   ReferentialSlug
	Errors Errors `json:"Errors,omitempty"`
}

func (referential *APIReferential) Validate() bool {
	valid := true
	if referential.Slug == "" {
		referential.Errors.Add("Slug", ERROR_BLANK)
		valid = false
	}
	return valid
}

func (referential *Referential) Id() ReferentialId {
	return referential.id
}

func (referential *Referential) Slug() ReferentialSlug {
	return referential.slug
}

// WIP: Interface ?
func (referential *Referential) CollectManager() CollectManagerInterface {
	return referential.collectManager
}

func (referential *Referential) Model() model.Model {
	return referential.model
}

func (referential *Referential) ModelGuardian() *ModelGuardian {
	return referential.modelGuardian
}

func (referential *Referential) Partners() Partners {
	return referential.partners
}

func (referential *Referential) Start() {
	referential.partners.Start()
	referential.modelGuardian.Start()
}

func (referential *Referential) Stop() {
	referential.partners.Stop()
	referential.modelGuardian.Stop()
}

func (referential *Referential) Save() (ok bool) {
	ok = referential.manager.Save(referential)
	return
}

func (referential *Referential) NewTransaction() *model.Transaction {
	return model.NewTransaction(referential.model)
}

func (referential *Referential) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"Id":   referential.id,
		"Slug": referential.slug,
	})
}

func (referential *Referential) Definition() *APIReferential {
	return &APIReferential{
		Id:     referential.id,
		Slug:   referential.slug,
		Errors: NewErrors(),
	}
}

func (referential *Referential) SetDefinition(apiReferential *APIReferential) {
	referential.id = apiReferential.Id
	referential.slug = apiReferential.Slug
}

type MemoryReferentials struct {
	model.UUIDConsumer

	byId map[ReferentialId]*Referential
}

func NewMemoryReferentials() *MemoryReferentials {
	return &MemoryReferentials{
		byId: make(map[ReferentialId]*Referential),
	}
}

func CurrentReferentials() Referentials {
	return referentials
}

func (manager *MemoryReferentials) New(slug ReferentialSlug) *Referential {
	referential := manager.new()
	referential.slug = slug
	return referential
}

func (manager *MemoryReferentials) new() *Referential {
	model := model.NewMemoryModel()
	partners := NewPartnerManager(model)
	referential := &Referential{
		manager:        manager,
		model:          model,
		partners:       partners,
		collectManager: NewCollectManager(partners),
	}
	referential.modelGuardian = NewModelGuardian(referential)
	return referential
}

func (manager *MemoryReferentials) Find(id ReferentialId) *Referential {
	referential, _ := manager.byId[id]
	return referential
}

func (manager *MemoryReferentials) FindBySlug(slug ReferentialSlug) *Referential {
	for _, referential := range manager.byId {
		if referential.slug == slug {
			return referential
		}
	}
	return nil
}

func (manager *MemoryReferentials) FindAll() (referentials []*Referential) {
	for _, referential := range manager.byId {
		referentials = append(referentials, referential)
	}
	return
}

func (manager *MemoryReferentials) Save(referential *Referential) bool {
	if referential.id == "" {
		referential.id = ReferentialId(manager.NewUUID())
	}
	referential.manager = manager
	manager.byId[referential.id] = referential
	return true
}

func (manager *MemoryReferentials) Delete(referential *Referential) bool {
	delete(manager.byId, referential.id)
	return true
}

func (manager *MemoryReferentials) Load() error {
	var selectReferentials []struct {
		Referential_id string
		Slug           string
	}
	_, err := model.Database.Select(&selectReferentials, "select * from referentials")
	if err != nil {
		return err
	}

	for _, r := range selectReferentials {
		referential := manager.new()
		referential.id = ReferentialId(r.Referential_id)
		referential.slug = ReferentialSlug(r.Slug)
		manager.Save(referential)
	}

	logger.Log.Debugf("Loaded Referentials from database")
	return nil
}

type ReferentialsConsumer struct {
	referentials Referentials
}

func (consumer *ReferentialsConsumer) SetReferentials(referentials Referentials) {
	consumer.referentials = referentials
}

func (consumer *ReferentialsConsumer) CurrentReferentials() Referentials {
	if consumer.referentials == nil {
		consumer.referentials = CurrentReferentials()
	}
	return consumer.referentials
}
