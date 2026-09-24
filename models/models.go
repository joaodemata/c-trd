package models

import (
	cmm "c_trd/common"
)

// RegistryList contains the memory maps for this entire package
var RegistryList = []cmm.ModelRegistry{
	{Name: "triggers", Ptr: &TriggersModel},
	{Name: "catalogs", Ptr: &CatalogsModel},
	{Name: "assets", Ptr: &AssetsModel},
	{Name: "opportunities", Ptr: &OpportunityModel},
	{Name: "providers", Ptr: &ProvidersModel},
	{Name: "conditions", Ptr: &ConditionsModel},
	{Name: "apis", Ptr: &ApisModel},
}
