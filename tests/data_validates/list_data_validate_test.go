package data_validates_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	cmm "c_trd/common"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Solo se prueban los casos que no consultan MongoDB: sin filtro de status o con tagStatus de un usuario sin privilegio
func TestListConditionsDataValidate(t *testing.T) {
	t.Parallel()

	visible := []primitive.ObjectID{cmm.GenericStatusActive.ID, cmm.GenericStatusInactive.ID}

	tests := []struct {
		name           string
		tagPrivilege   string // vacio: usuario sin registro
		filters        fv.ListConditionsFormat
		wantNext       bool
		wantIDStatuses []primitive.ObjectID
		wantTracking   string
	}{
		{name: "sin sesion ve activas e inactivas", wantNext: true, wantIDStatuses: visible},
		{name: "usuario generico ve activas e inactivas", tagPrivilege: cmm.UserPrivilegeGenericUser.Tag, wantNext: true, wantIDStatuses: visible},
		{name: "super usuario ve todas", tagPrivilege: cmm.UserPrivilegeSuperUser.Tag, wantNext: true, wantIDStatuses: []primitive.ObjectID{}},
		{name: "agente ve todas", tagPrivilege: cmm.UserPrivilegeAgent.Tag, wantNext: true, wantIDStatuses: []primitive.ObjectID{}},
		{name: "sin sesion con tagStatus", filters: fv.ListConditionsFormat{TagStatus: "STATUS-ACTIVE"}, wantTracking: "DVLIS006"},
		{name: "generico con tagStatus", tagPrivilege: cmm.UserPrivilegeGenericUser.Tag, filters: fv.ListConditionsFormat{TagStatus: "STATUS-ACTIVE"}, wantTracking: "DVLIS006"},
		{name: "agente con tagStatus", tagPrivilege: cmm.UserPrivilegeAgent.Tag, filters: fv.ListConditionsFormat{TagStatus: "STATUS-ACTIVE"}, wantTracking: "DVLIS006"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/list/conditions", nil)
			var session *models.SessionsModelType
			if tt.tagPrivilege != "" {
				session = &models.SessionsModelType{RecordOwnerData: models.RecordOwnerData{TagTypePrivilege: tt.tagPrivilege}}
			}
			filters := tt.filters
			req = cmm.SetInRequestContext(req, "authSession", session)
			req = cmm.SetInRequestContext(req, "filters", &filters)
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			dv.ListConditionsDataValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				res := testutil.DecodeResponse(t, rec)
				if res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				if rec.Code != http.StatusForbidden {
					t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusForbidden)
				}
				return
			}

			idStatuses, errCtx := cmm.GetFromRequestContext[[]primitive.ObjectID](spy.Request, "idStatuses")
			testutil.RequireEmptyError(t, errCtx)
			if !slices.Equal(idStatuses, tt.wantIDStatuses) {
				t.Errorf("idStatuses = %v, want %v", idStatuses, tt.wantIDStatuses)
			}
		})
	}
}
