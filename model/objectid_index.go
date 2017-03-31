package model

type ObjectIdIndex struct {
	byObjectid   map[ObjectID]ModelId
	byIdentifier map[ModelId]ObjectIDs
}

func NewObjectIdIndex() *ObjectIdIndex {
	return &ObjectIdIndex{
		byObjectid:   make(map[ObjectID]ModelId),
		byIdentifier: make(map[ModelId]ObjectIDs),
	}
}

func (index *ObjectIdIndex) Index(modelId ModelId, model ModelInstance) {
	currentIndexable, ok := index.byIdentifier[modelId]
	if ok {
		for indexedKind, indexedObjectid := range currentIndexable {
			modelObjectid, ok := model.ObjectID(indexedKind)
			if !ok || modelObjectid.Value() != indexedObjectid.Value() {
				delete(index.byObjectid, indexedObjectid)
			}
		}
	}

	if index.byIdentifier[modelId] == nil {
		index.byIdentifier[modelId] = make(ObjectIDs)
	}

	for _, objectid := range model.ObjectIDs() {
		index.byObjectid[objectid] = modelId
		index.byIdentifier[modelId][objectid.Kind()] = objectid
	}
}

func (index *ObjectIdIndex) Find(objectid ObjectID) (ModelId, bool) {
	modelId, ok := index.byObjectid[objectid]
	return modelId, ok
}

func (index *ObjectIdIndex) Delete(modelId ModelId) {
	currentIndexable, ok := index.byIdentifier[modelId]
	if !ok {
		return
	}

	for _, objectid := range currentIndexable {
		delete(index.byObjectid, objectid)
	}
	delete(index.byIdentifier, modelId)
}
