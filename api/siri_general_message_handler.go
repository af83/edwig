package api

import (
	"fmt"
	"net/http"
	"time"

	"bitbucket.org/enroute-mobi/ara/audit"
	"bitbucket.org/enroute-mobi/ara/clock"
	"bitbucket.org/enroute-mobi/ara/core"
	"bitbucket.org/enroute-mobi/ara/logger"
	"bitbucket.org/enroute-mobi/ara/siri"
)

type SIRIGeneralMessageRequestHandler struct {
	xmlRequest *siri.XMLGetGeneralMessage
}

func (handler *SIRIGeneralMessageRequestHandler) RequestorRef() string {
	return handler.xmlRequest.RequestorRef()
}

func (handler *SIRIGeneralMessageRequestHandler) ConnectorType() string {
	return core.SIRI_GENERAL_MESSAGE_REQUEST_BROADCASTER
}

func (handler *SIRIGeneralMessageRequestHandler) Respond(connector core.Connector, rw http.ResponseWriter, message *audit.BigQueryMessage) {
	logger.Log.Debugf("General Message %s\n", handler.xmlRequest.MessageIdentifier())

	t := clock.DefaultClock().Now()

	tmp := connector.(*core.SIRIGeneralMessageRequestBroadcaster)
	response, _ := tmp.Situations(handler.xmlRequest, message)
	xmlResponse, err := response.BuildXML()
	if err != nil {
		siriError("InternalServiceError", fmt.Sprintf("Internal Error: %v", err), rw)
		return
	}

	// Wrap soap and send response
	soapEnvelope := siri.NewSOAPEnvelopeBuffer()
	soapEnvelope.WriteXML(xmlResponse)

	n, err := soapEnvelope.WriteTo(rw)
	if err != nil {
		siriError("InternalServiceError", fmt.Sprintf("Internal Error: %v", err), rw)
		return
	}

	message.Type = "GeneralMessageRequest"
	message.RequestRawMessage = handler.xmlRequest.RawXML()
	message.ResponseRawMessage = xmlResponse
	message.ResponseSize = n
	message.ProcessingTime = time.Since(t).Seconds()
	audit.CurrentBigQuery().WriteEvent(message)
}
