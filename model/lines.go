package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type LineId string

type LineAttributes struct {
	ObjectId ObjectID
	Name     string
}

type Line struct {
	ObjectIDConsumer
	model Model

	id LineId

	Name       string
	Attributes Attributes
	References References
}

func NewLine(model Model) *Line {
	line := &Line{
		model:      model,
		Attributes: NewAttributes(),
		References: NewReferences(),
	}

	line.objectids = make(ObjectIDs)
	return line
}

func (line *Line) Id() LineId {
	return line.id
}

func (line *Line) FillLine(lineMap map[string]interface{}) {
	if line.id != "" {
		lineMap["Id"] = line.id
	}

	if line.Name != "" {
		lineMap["Name"] = line.Name
	}

	if !line.Attributes.IsEmpty() {
		lineMap["Attributes"] = line.Attributes
	}

	if !line.References.IsEmpty() {
		lineMap["References"] = line.References
	}
}

func (line *Line) MarshalJSON() ([]byte, error) {
	lineMap := make(map[string]interface{})

	if !line.ObjectIDs().Empty() {
		lineMap["ObjectIDs"] = line.ObjectIDs()
	}
	line.FillLine(lineMap)
	return json.Marshal(lineMap)
}

func (line *Line) UnmarshalJSON(data []byte) error {
	type Alias Line

	aux := &struct {
		ObjectIDs map[string]string
		*Alias
	}{
		Alias: (*Alias)(line),
	}

	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	if aux.ObjectIDs != nil {
		line.ObjectIDConsumer.objectids = NewObjectIDsFromMap(aux.ObjectIDs)
	}

	return nil
}

func (line *Line) Attribute(key string) (string, bool) {
	value, present := line.Attributes[key]
	return value, present
}

func (line *Line) Reference(key string) (Reference, bool) {
	value, present := line.References[key]
	return value, present
}

func (line *Line) Save() (ok bool) {
	ok = line.model.Lines().Save(line)
	return
}

type MemoryLines struct {
	UUIDConsumer

	model Model

	byIdentifier map[LineId]*Line
}

type Lines interface {
	UUIDInterface

	New() Line
	Find(id LineId) (Line, bool)
	FindByObjectId(objectid ObjectID) (Line, bool)
	FindAll() []Line
	Save(line *Line) bool
	Delete(line *Line) bool
}

func NewMemoryLines() *MemoryLines {
	return &MemoryLines{
		byIdentifier: make(map[LineId]*Line),
	}
}

func (manager *MemoryLines) New() Line {
	line := NewLine(manager.model)
	return *line
}

func (manager *MemoryLines) Find(id LineId) (Line, bool) {
	line, ok := manager.byIdentifier[id]
	if ok {
		return *line, true
	} else {
		return Line{}, false
	}
}

func (manager *MemoryLines) FindByObjectId(objectid ObjectID) (Line, bool) {
	for _, line := range manager.byIdentifier {
		lineObjectId, _ := line.ObjectID(objectid.Kind())
		if lineObjectId.Value() == objectid.Value() {
			return *line, true
		}
	}
	return Line{}, false
}

func (manager *MemoryLines) FindAll() (lines []Line) {
	if len(manager.byIdentifier) == 0 {
		return []Line{}
	}
	for _, line := range manager.byIdentifier {
		lines = append(lines, *line)
	}
	return
}

func (manager *MemoryLines) Save(line *Line) bool {
	if line.Id() == "" {
		line.id = LineId(manager.NewUUID())
	}
	line.model = manager.model
	manager.byIdentifier[line.Id()] = line
	return true
}

func (manager *MemoryLines) Delete(line *Line) bool {
	delete(manager.byIdentifier, line.Id())
	return true
}

func (manager *MemoryLines) Load(referentialId string) error {
	var selectLines []struct {
		Id            string
		ReferentialId string `db:"referential_id"`
		Name          sql.NullString
		ObjectIDs     sql.NullString `db:"object_ids"`
		Attributes    sql.NullString
		References    sql.NullString `db:"siri_references"`
	}
	sqlQuery := fmt.Sprintf("select * from lines where referential_id = '%s'", referentialId)
	_, err := Database.Select(&selectLines, sqlQuery)
	if err != nil {
		return err
	}
	for _, sl := range selectLines {
		line := manager.New()
		line.id = LineId(sl.Id)
		if sl.Name.Valid {
			line.Name = sl.Name.String
		}

		if sl.Attributes.Valid && len(sl.Attributes.String) > 0 {
			if err = json.Unmarshal([]byte(sl.Attributes.String), &line.Attributes); err != nil {
				return err
			}
		}

		if sl.References.Valid && len(sl.References.String) > 0 {
			if err = json.Unmarshal([]byte(sl.References.String), &line.References); err != nil {
				return err
			}
		}

		if sl.ObjectIDs.Valid && len(sl.ObjectIDs.String) > 0 {
			objectIdMap := make(map[string]string)
			if err = json.Unmarshal([]byte(sl.ObjectIDs.String), &objectIdMap); err != nil {
				return err
			}
			line.objectids = NewObjectIDsFromMap(objectIdMap)
		}

		manager.Save(&line)
	}
	return nil
}
