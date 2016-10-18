package siri

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jbowtie/gokogiri/xml"
)

type XMLStructure struct {
	node xml.Node
}

func (xmlStruct *XMLStructure) findNode(localName string) xml.Node {
	xpath := fmt.Sprintf("//*[local-name()='%s']", localName)
	nodes, err := xmlStruct.node.Search(xpath)
	if err != nil {
		log.Fatal(err)
	}
	return nodes[0]
}

// TODO: See how to handle errors
func (xmlStruct *XMLStructure) findStringChildContent(localName string) string {
	node := xmlStruct.findNode(localName)
	return strings.TrimSpace(node.Content())
}

func (xmlStruct *XMLStructure) findTimeChildContent(localName string) time.Time {
	node := xmlStruct.findNode(localName)
	t, err := time.Parse("2006-01-02T15:04:05.000Z07:00", strings.TrimSpace(node.Content()))
	if err != nil {
		log.Fatal(err)
	}
	return t
}

func (xmlStruct *XMLStructure) findBoolChildContent(localName string) bool {
	node := xmlStruct.findNode(localName)
	s, err := strconv.ParseBool(strings.TrimSpace(node.Content()))
	if err != nil {
		log.Fatal(err)
	}
	return s
}