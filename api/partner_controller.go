package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/af83/edwig/core"
	"github.com/af83/edwig/logger"
)

type PartnerController struct {
	referential *core.Referential
}

func NewPartnerController(referential *core.Referential) ControllerInterface {
	return &Controller{
		restfulRessource: &PartnerController{
			referential: referential,
		},
	}
}

func (controller *PartnerController) subscriptions(response http.ResponseWriter, requestData *RequestData) {
	partner := controller.findPartner(requestData.Id)
	if partner == nil {
		http.Error(response, fmt.Sprintf("Partner not found: %s", requestData.Id), 500)
		return
	}
	logger.Log.Debugf("Get partner %s for Subscriptions", requestData.Id)

	subscriptions := partner.Subscriptions()
	jsonBytes, _ := json.Marshal(subscriptions.FindAll())
	response.Write(jsonBytes)
}

func (controller *PartnerController) Action(response http.ResponseWriter, requestData *RequestData) {
	if requestData.Action == "subscriptions" {
		controller.subscriptions(response, requestData)
		return
	}
	http.Error(response, fmt.Sprintf("Action not supported: %s", requestData.Action), 500)
}

func (controller *PartnerController) findPartner(identifier string) *core.Partner {
	partner, ok := controller.referential.Partners().FindBySlug(core.PartnerSlug(identifier))
	if ok {
		return partner
	}
	return controller.referential.Partners().Find(core.PartnerId(identifier))
}

func (controller *PartnerController) Index(response http.ResponseWriter) {
	logger.Log.Debugf("Partners Index")

	jsonBytes, _ := json.Marshal(controller.referential.Partners().FindAll())
	response.Write(jsonBytes)
}

func (controller *PartnerController) Show(response http.ResponseWriter, identifier string) {
	partner := controller.findPartner(identifier)
	if partner == nil {
		http.Error(response, fmt.Sprintf("Partner not found: %s", identifier), 500)
		return
	}
	logger.Log.Debugf("Get partner %s", identifier)

	jsonBytes, _ := partner.MarshalJSON()
	response.Write(jsonBytes)
}

func (controller *PartnerController) Delete(response http.ResponseWriter, identifier string) {
	partner := controller.findPartner(identifier)
	if partner == nil {
		http.Error(response, fmt.Sprintf("Partner not found: %s", identifier), 500)
		return
	}
	logger.Log.Debugf("Delete partner %s", identifier)

	jsonBytes, _ := partner.MarshalJSON()
	controller.referential.Partners().Delete(partner)
	response.Write(jsonBytes)
}

func (controller *PartnerController) Update(response http.ResponseWriter, identifier string, body []byte) {
	partner := controller.findPartner(identifier)
	if partner == nil {
		http.Error(response, fmt.Sprintf("Partner not found: %s", identifier), 500)
		return
	}

	logger.Log.Debugf("Update partner %s: %s", identifier, string(body))

	apiPartner := partner.Definition()
	err := json.Unmarshal(body, apiPartner)
	if err != nil {
		http.Error(response, fmt.Sprintf("Invalid request: can't parse request body: %v", err), 400)
		return
	}
	if apiPartner.Id != partner.Id() {
		http.Error(response, "Invalid request (Id specified)", 400)
		return
	}

	if !apiPartner.Validate() {
		jsonBytes, _ := json.Marshal(apiPartner)
		response.WriteHeader(http.StatusBadRequest)
		response.Write(jsonBytes)
		return
	}

	partner.SetDefinition(apiPartner)
	partner.Save()
	jsonBytes, _ := partner.MarshalJSON()
	response.Write(jsonBytes)
}

func (controller *PartnerController) Create(response http.ResponseWriter, body []byte) {
	logger.Log.Debugf("Create partner: %s", string(body))

	partner := controller.referential.Partners().New("")
	apiPartner := partner.Definition()
	err := json.Unmarshal(body, apiPartner)
	if err != nil {
		http.Error(response, fmt.Sprintf("Invalid request: can't parse request body: %v", err), 400)
		return
	}
	if apiPartner.Id != "" {
		http.Error(response, "Invalid request (Id specified)", 400)
		return
	}

	if !apiPartner.Validate() {
		jsonBytes, _ := json.Marshal(apiPartner)
		response.WriteHeader(http.StatusBadRequest)
		response.Write(jsonBytes)
		return
	}

	partner.SetDefinition(apiPartner)
	controller.referential.Partners().Save(partner)
	jsonBytes, _ := partner.MarshalJSON()
	response.Write(jsonBytes)
}
