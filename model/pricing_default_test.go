package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultVendorMappingClassifiesMediaModels(t *testing.T) {
	vendorMap := map[int]*Vendor{
		1: {Id: 1, Name: "OpenAI"},
		2: {Id: 2, Name: "Google"},
		3: {Id: 3, Name: "即梦"},
	}
	metaMap := map[string]*Model{}
	abilities := []AbilityWithChannel{
		{Ability: Ability{Model: "sora-2"}},
		{Ability: Ability{Model: "nano-banana2-1k"}},
		{Ability: Ability{Model: "omni-fast"}},
		{Ability: Ability{Model: "veo-3-1"}},
		{Ability: Ability{Model: "seedance-2.0"}},
	}

	initDefaultVendorMapping(metaMap, vendorMap, abilities)

	assert.Equal(t, 1, metaMap["sora-2"].VendorID)
	assert.Equal(t, 2, metaMap["nano-banana2-1k"].VendorID)
	assert.Equal(t, 2, metaMap["omni-fast"].VendorID)
	assert.Equal(t, 2, metaMap["veo-3-1"].VendorID)
	assert.Equal(t, 3, metaMap["seedance-2.0"].VendorID)
}
