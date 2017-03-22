package siri

import (
	"bytes"
	"text/template"
	"time"
)

type SIRIStopPointsDiscoveryResponse struct {
	Address                   string
	ProducerRef               string
	RequestMessageRef         string
	ResponseMessageIdentifier string
	Status                    bool
	ResponseTimestamp         time.Time

	AnnotatedStopPoints []*SIRIAnnotatedStopPoint
}

type SIRIAnnotatedStopPoint struct {
	StopPointRef  string
	StopPointName string
}

const stopDiscoveryResponseTemplate = `
<ns8:StopPointsDiscoveryResponse xmlns:ns8="http://wsdl.siri.org.uk" xmlns:ns3="http://www.siri.org.uk/siri" xmlns:ns4="http://www.ifopt.org.uk/acsb" xmlns:ns5="http://www.ifopt.org.uk/ifopt" xmlns:ns6="http://datex2.eu/schema/2_0RC1/2_0" xmlns:ns7="http://scma/siri" xmlns:ns9="http://wsdl.siri.org.uk/siri">
   <Answer version="2.0">
      <ns3:ResponseTimestamp>{{.ResponseTimestamp}}</ns3:ResponseTimestamp>
      <ns3:Status>{{.Status}}</ns3:Status>{{range .AnnotatedStopPoints}}
      <ns3:AnnotatedStopPointRef>
         <ns3:StopPointRef>{{.StopPointRef}}</ns3:StopPointRef>
         <ns3:StopPointName>{{.StopPointName}}</ns3:StopPointName>
      </ns3:AnnotatedStopPointRef>{{ end }}
   </Answer>
   <AnswerExtension />
</ns8:StopPointsDiscoveryResponse>`

func (response *SIRIStopPointsDiscoveryResponse) BuildXML() (string, error) {
	var buffer bytes.Buffer
	var siriResponse = template.Must(template.New("siriResponse").Parse(stopDiscoveryResponseTemplate))
	if err := siriResponse.Execute(&buffer, response); err != nil {
		return "", err
	}
	return buffer.String(), nil
}