package models

import (
	cmm "c_trd/common"
)

// RegistryList contains the memory maps for this entire package
var RegistryList = []cmm.ModelRegistry{
	{Name: "triggers", Ptr: &TriggersModel},
}