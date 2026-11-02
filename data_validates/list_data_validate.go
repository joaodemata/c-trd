package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/services"
	"net/http"
)

// ListTriggersDataValidate valida que el tagStatus exista en el catalogo de status genericos y que solo se pida un
// status distinto de activo junto con showReferences
func ListTriggersDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		filters, err := cmm.GetFromRequestContext[*fv.ListTriggersFormat](r, "filters")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVLIS001", nil, err)
			return
		}

		// Sin tagStatus no hay nada que validar
		if filters.TagStatus == "" {
			next.ServeHTTP(w, r)
			return
		}

		status, err := services.GetCatalogByTagService(filters.TagStatus, cmm.CatalogTypeGenericStatus)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if status.ID.IsZero() {
			res.Error("FAIL", "El tagStatus enviado no existe", "DVLIS002", filters.TagStatus, cmm.NewEmptyErrorHandler())
			return
		}

		// Los triggers inactivos y reemplazados solo se listan con showReferences. Los pendientes de aprobacion y los no
		// aprobados si, para revisar lo que crearon los agentes
		reviewable := status.Tag == cmm.GenericStatusPendingForApprove.Tag || status.Tag == cmm.GenericStatusNotApproved.Tag
		listable := status.Tag == cmm.GenericStatusActive.Tag || reviewable
		if !listable && !filters.ShowReferences {
			res.Error("FAIL", "Para listar triggers inactivos o reemplazados se debe enviar showReferences=true", "DVLIS003", filters.TagStatus, cmm.NewEmptyErrorHandler())
			return
		}

		next.ServeHTTP(w, r)
	})
}
