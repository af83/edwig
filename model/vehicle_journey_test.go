package model

import "testing"

func Test_VehicleJourney_Id(t *testing.T) {
	vehicleJourney := VehicleJourney{
		id: "6ba7b814-9dad-11d1-0-00c04fd430c8",
	}

	if vehicleJourney.Id() != "6ba7b814-9dad-11d1-0-00c04fd430c8" {
		t.Errorf("VehicleJourney.Id() returns wrong value, got: %s, required: %s", vehicleJourney.Id(), "6ba7b814-9dad-11d1-0-00c04fd430c8")
	}
}

// WIP: Determine what to return in JSON
func Test_VehicleJourney_MarshalJSON(t *testing.T) {
	vehicleJourney := VehicleJourney{
		id: "6ba7b814-9dad-11d1-0-00c04fd430c8",
	}
	expected := `{"Id":"6ba7b814-9dad-11d1-0-00c04fd430c8"}`
	jsonBytes, err := vehicleJourney.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	jsonString := string(jsonBytes)
	if jsonString != expected {
		t.Errorf("VehicleJourney.MarshalJSON() returns wrong json:\n got: %s\n want: %s", jsonString, expected)
	}
}

func Test_VehicleJourney_Save(t *testing.T) {
	model := NewMemoryModel()
	vehicleJourney := model.VehicleJourneys().New()
	objectid := NewObjectID("kind", "value")
	vehicleJourney.SetObjectID(objectid)

	if vehicleJourney.model != model {
		t.Errorf("New vehicleJourney model should be memoryVehicleJourneys model")
	}

	ok := vehicleJourney.Save()
	if !ok {
		t.Errorf("vehicleJourney.Save() should succeed")
	}
	_, ok = model.VehicleJourneys().Find(vehicleJourney.Id())
	if !ok {
		t.Errorf("New VehicleJourney should be found in memoryVehicleJourneys")
	}
	_, ok = model.VehicleJourneys().FindByObjectId(objectid)
	if !ok {
		t.Errorf("New VehicleJourney should be found by objectid in memoryVehicleJourneys")
	}
}

func Test_VehicleJourney_ObjectId(t *testing.T) {
	vehicleJourney := VehicleJourney{
		id: "6ba7b814-9dad-11d1-0-00c04fd430c8",
	}
	vehicleJourney.objectids = make(ObjectIDs)
	objectid := NewObjectID("kind", "value")
	vehicleJourney.SetObjectID(objectid)

	foundObjectId, ok := vehicleJourney.ObjectID("kind")
	if !ok {
		t.Errorf("ObjectID should return true if ObjectID exists")
	}
	if foundObjectId.Value() != objectid.Value() {
		t.Errorf("ObjectID should return a correct ObjectID:\n got: %v\n want: %v", foundObjectId, objectid)
	}

	_, ok = vehicleJourney.ObjectID("wrongkind")
	if ok {
		t.Errorf("ObjectID should return false if ObjectID doesn't exist")
	}

	if len(vehicleJourney.ObjectIDs()) != 1 {
		t.Errorf("ObjectIDs should return an array with set ObjectIDs, got: %v", vehicleJourney.ObjectIDs())
	}
}

func Test_MemoryVehicleJourneys_New(t *testing.T) {
	vehicleJourneys := NewMemoryVehicleJourneys()

	vehicleJourney := vehicleJourneys.New()
	if vehicleJourney.Id() != "" {
		t.Errorf("New VehicleJourney identifier should be an empty string, got: %s", vehicleJourney.Id())
	}
}

func Test_MemoryVehicleJourneys_Save(t *testing.T) {
	vehicleJourneys := NewMemoryVehicleJourneys()

	vehicleJourney := vehicleJourneys.New()

	if success := vehicleJourneys.Save(&vehicleJourney); !success {
		t.Errorf("Save should return true")
	}

	if vehicleJourney.Id() == "" {
		t.Errorf("New VehicleJourney identifier shouldn't be an empty string")
	}
}

func Test_MemoryVehicleJourneys_Find_NotFound(t *testing.T) {
	vehicleJourneys := NewMemoryVehicleJourneys()
	_, ok := vehicleJourneys.Find("6ba7b814-9dad-11d1-0-00c04fd430c8")
	if ok {
		t.Errorf("Find should return false when VehicleJourney isn't found")
	}
}

func Test_MemoryVehicleJourneys_Find(t *testing.T) {
	vehicleJourneys := NewMemoryVehicleJourneys()

	existingVehicleJourney := vehicleJourneys.New()
	vehicleJourneys.Save(&existingVehicleJourney)

	vehicleJourneyId := existingVehicleJourney.Id()

	vehicleJourney, ok := vehicleJourneys.Find(vehicleJourneyId)
	if !ok {
		t.Errorf("Find should return true when VehicleJourney is found")
	}
	if vehicleJourney.Id() != vehicleJourneyId {
		t.Errorf("Find should return a VehicleJourney with the given Id")
	}
}

func Test_MemoryVehicleJourneys_FindAll(t *testing.T) {
	vehicleJourneys := NewMemoryVehicleJourneys()

	for i := 0; i < 5; i++ {
		existingVehicleJourney := vehicleJourneys.New()
		vehicleJourneys.Save(&existingVehicleJourney)
	}

	foundVehicleJourneys := vehicleJourneys.FindAll()

	if len(foundVehicleJourneys) != 5 {
		t.Errorf("FindAll should return all vehicleJourneys")
	}
}

func Test_MemoryVehicleJourneys_Delete(t *testing.T) {
	vehicleJourneys := NewMemoryVehicleJourneys()
	existingVehicleJourney := vehicleJourneys.New()
	objectid := NewObjectID("kind", "value")
	existingVehicleJourney.SetObjectID(objectid)
	vehicleJourneys.Save(&existingVehicleJourney)

	vehicleJourneys.Delete(&existingVehicleJourney)

	_, ok := vehicleJourneys.Find(existingVehicleJourney.Id())
	if ok {
		t.Errorf("Deleted VehicleJourney should not be findable")
	}
	_, ok = vehicleJourneys.FindByObjectId(objectid)
	if ok {
		t.Errorf("Deleted VehicleJourney should not be findable by objectid")
	}
}