package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/services"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

// ListConditionsDataValidate arma los status que se pueden listar (idStatuses). Sin sesion o sin privilegio solo se
// listan las condiciones activas o inactivas y el filtro idStatus debe ser uno de esos. El super usuario lista todas y es
// el unico que puede filtrar por tagStatus. El agente tambien ve todas para reutilizar las pendientes y no volver a
// crear las no aprobadas. Va despues de OptionalAuthDataValidate y ListConditionsFormatValidate
func ListConditionsDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVLIS004", nil, err)
			return
		}

		filters, err := cmm.GetFromRequestContext[*fv.ListConditionsFormat](r, "filters")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVLIS005", nil, err)
			return
		}

		privilege := ""
		if authSession != nil {
			privilege = authSession.TagTypePrivilege
		}
		isSuperUser := privilege == cmm.UserPrivilegeSuperUser.Tag
		seesAll := isSuperUser || privilege == cmm.UserPrivilegeAgent.Tag

		if filters.TagStatus != "" && !isSuperUser {
			res.Send(http.StatusForbidden, "FAIL", "Solo el super usuario puede filtrar por tagStatus", "DVLIS006", nil)
			return
		}

		// Sin filtro: el super usuario y el agente ven todas, el resto solo las activas o inactivas
		if filters.IdStatus == "" && filters.TagStatus == "" {
			idStatuses := []primitive.ObjectID{}
			if !seesAll {
				idStatuses = []primitive.ObjectID{cmm.GenericStatusActive.ID, cmm.GenericStatusInactive.ID}
			}
			r = cmm.SetInRequestContext(r, "idStatuses", idStatuses)
			next.ServeHTTP(w, r)
			return
		}

		var status models.CatalogsModelType
		if filters.TagStatus != "" {
			status, err = services.GetCatalogByTagService(filters.TagStatus, cmm.CatalogTypeGenericStatus)
		} else {
			status, err = findCatalog(filters.IdStatus, cmm.CatalogTypeGenericStatus)
		}
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if status.ID.IsZero() {
			res.Error("FAIL", "El status enviado no existe", "DVLIS007", nil, cmm.NewEmptyErrorHandler())
			return
		}

		visible := status.ID == cmm.GenericStatusActive.ID || status.ID == cmm.GenericStatusInactive.ID
		if !visible && !seesAll {
			res.Send(http.StatusForbidden, "FAIL", "Solo se pueden listar condiciones activas o inactivas", "DVLIS008", nil)
			return
		}

		r = cmm.SetInRequestContext(r, "idStatuses", []primitive.ObjectID{status.ID})
		next.ServeHTTP(w, r)
	})
}
