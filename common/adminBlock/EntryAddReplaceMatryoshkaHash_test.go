package adminBlock_test

import (
	"testing"

	. "github.com/FactomProject/factomd/common/adminBlock"
	"github.com/FactomProject/factomd/common/constants"
	"github.com/FactomProject/factomd/testHelper"
)

func TestAddReplaceMatryoshkaHashGetHash(t *testing.T) {
	a := new(AddReplaceMatryoshkaHash)
	h := a.Hash()
	expected := "dc48a742ae32cfd66352372d6120ed14d6629fc166246b05ff8b03e23804701f"
	if h.String() != expected {
		t.Errorf("Wrong hash returned - %v vs %v", h.String(), expected)
	}
}

func TestUnmarshalNilAddReplaceMatryoshkaHash(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic caught during the test - %v", r)
		}
	}()

	a := new(AddReplaceMatryoshkaHash)
	err := a.UnmarshalBinary(nil)
	if err == nil {
		t.Errorf("Error is nil when it shouldn't be")
	}

	err = a.UnmarshalBinary([]byte{})
	if err == nil {
		t.Errorf("Error is nil when it shouldn't be")
	}
}

func TestAddReplaceMatryoshkaHashMarshalUnmarshal(t *testing.T) {
	identity := testHelper.NewRepeatingHash(0xAB)
	mhash := testHelper.NewRepeatingHash(0xCD)

	rmh := NewAddReplaceMatryoshkaHash(identity, mhash)
	if rmh.Type() != constants.TYPE_ADD_MATRYOSHKA {
		t.Errorf("Invalid type")
	}
	if rmh.IdentityChainID.IsSameAs(identity) == false {
		t.Errorf("Invalid IdentityChainID")
	}
	if rmh.MHash.IsSameAs(mhash) == false {
		t.Errorf("Invalid MHash")
	}
	tmp2, err := rmh.MarshalBinary()
	if err != nil {
		t.Error(err)
	}

	rmh = new(AddReplaceMatryoshkaHash)
	err = rmh.UnmarshalBinary(tmp2)
	if err != nil {
		t.Error(err)
	}
	if rmh.Type() != constants.TYPE_ADD_MATRYOSHKA {
		t.Errorf("Invalid type")
	}
	if rmh.IdentityChainID.IsSameAs(identity) == false {
		t.Errorf("Invalid IdentityChainID")
	}
	if rmh.MHash.IsSameAs(mhash) == false {
		t.Errorf("Invalid MHash")
	}
}
