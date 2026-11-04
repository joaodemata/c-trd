package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// CreateOpportunityService inserts a new document into the opportunities collection
func CreateOpportunityService(opportunity models.OpportunitiesModelType) (string, *cmm.ErrorHandler) {
	// 1. Set default metadata values for the new document
	if opportunity.ID == primitive.NilObjectID {
		opportunity.ID = primitive.NewObjectID()
	}

	// 2. Set timeout context for the query //TODO: crear un ctx generico para las operaciones de bd
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Execute query
	result, err := models.OpportunitiesModel.InsertOne(ctx, opportunity)
	if err != nil {
		// El indice unico uniq_opp_trigger_open rechazo el insert: ya hay una oportunidad en curso o activa del trigger
		if mongo.IsDuplicateKeyError(err) {
			return "", cmm.NewErrorHandler(err, "Ya existe una oportunidad en curso o activa para este trigger.", cmm.LevelError, "SOPRE021")
		}

		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SOPRE001")
	}

	// 4. Extract and parse the inserted ID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "SOPRE002")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}

// UpdateOpportunitiesStatus actualiza las oportunidades
func UpdateOpportunitiesStatus(ctx context.Context, idCondition primitive.ObjectID, newIDStatus primitive.ObjectID, newTagStatus string, newStatus string,
) *cmm.ErrorHandler {
	// filter
	filter := bson.M{"conditions.id_condition": idCondition}
	// Update
	update := bson.M{
		"$set": bson.M{
			"conditions.$.id_status":  newIDStatus,
			"conditions.$.tag_status": newTagStatus,
			"conditions.$.status":     newStatus,
			"last_updated_at":         time.Now().UTC(), // Actualiza la fecha a nivel raíz del documento
		},
	}

	// Usa el ctx (que será el SessionContext durante la transacción)
	result, err := models.OpportunitiesModel.UpdateMany(ctx, filter, update)
	// Check error
	if err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando las oportunidades.", cmm.LevelDatabase, "SOPRE003")
	}
	// Chequeamos que algo se haya actualizado
	if result.ModifiedCount < 1 {
		return cmm.NewErrorHandler(err, "Ningun registro actualizado.", cmm.LevelDatabase, "SOPRE004")
	}

	return cmm.NewEmptyErrorHandler()
}

// ExpireOpportunitiesService cambia a EXPIRED las oportunidades en curso cuya fecha de expiracion ya paso.
// Cada documento se actualiza de forma atomica, asi no se pisa una oportunidad que cambio de status
func ExpireOpportunitiesService(now time.Time) (int64, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{
		"tag_status":      cmm.OpportunityStatusOngoing.Tag,
		"logical_delete":  false,
		"expiration_date": bson.M{"$lte": now},
	}
	update := bson.M{
		"$set": bson.M{
			"id_status":       cmm.OpportunityStatusExpired.ID,
			"tag_status":      cmm.OpportunityStatusExpired.Tag,
			"status":          cmm.OpportunityStatusExpired.Name,
			"last_updated_at": now,
		},
	}

	result, err := models.OpportunitiesModel.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, cmm.NewErrorHandler(err, "Error expirando las oportunidades.", cmm.LevelDatabase, "SOPRE020")
	}

	return result.ModifiedCount, cmm.NewEmptyErrorHandler()
}

// CancelOpportunityService cambia a CANCELED una oportunidad en curso o activa y guarda la razon de cancelacion y el
// precio en price_achieved. El filtro vuelve a validar el status, asi no se cancela una que cambio desde que se consulto
func CancelOpportunityService(idOpportunity primitive.ObjectID, cancelReason models.CatalogsModelType, priceAchieved float64, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{
		"_id": idOpportunity,
		"tag_status": bson.M{"$in": []string{
			cmm.OpportunityStatusOngoing.Tag,
			cmm.OpportunityStatusActive.Tag,
		}},
		"logical_delete": false,
	}
	update := bson.M{
		"$set": bson.M{
			"id_status":         cmm.OpportunityStatusCanceled.ID,
			"tag_status":        cmm.OpportunityStatusCanceled.Tag,
			"status":            cmm.OpportunityStatusCanceled.Name,
			"price_achieved":    priceAchieved,
			"id_cancel":         cancelReason.ID,
			"tag_cancel_reason": cancelReason.Tag,
			"cancel_reason":     cancelReason.Name,
			"last_updated_at":   now,
		},
	}

	result, err := models.OpportunitiesModel.UpdateOne(ctx, filter, update)
	if err != nil {
		return cmm.NewErrorHandler(err, "Error cancelando la oportunidad.", cmm.LevelDatabase, "SOPRE048")
	}

	if result.MatchedCount < 1 {
		return cmm.NewErrorHandler(fmt.Errorf("opportunity %s", idOpportunity.Hex()), "La oportunidad ya no esta en curso ni activa.", cmm.LevelError, "SOPRE049")
	}

	return cmm.NewEmptyErrorHandler()
}

// finishedOpportunitiesFilter oportunidades activas cuya fecha de expiracion ya paso
func finishedOpportunitiesFilter(now time.Time) bson.M {
	return bson.M{
		"tag_status":      cmm.OpportunityStatusActive.Tag,
		"logical_delete":  false,
		"expiration_date": bson.M{"$lte": now},
	}
}

// OpportunityFinish oportunidad que pasa a FINISHED con el precio de mercado de su asset (nil si no se obtuvo)
type OpportunityFinish struct {
	IDOpportunity primitive.ObjectID
	PriceAchieved *float64
}

// FinishOpportunitiesService cambia a FINISHED las oportunidades y guarda su precio en price_achieved. Cada oportunidad
// se vuelve a filtrar con finishedOpportunitiesFilter, asi no se finaliza una que cambio desde que se listo
func FinishOpportunitiesService(finishes []OpportunityFinish, now time.Time) (int64, *cmm.ErrorHandler) {
	if len(finishes) == 0 {
		return 0, cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	writes := make([]mongo.WriteModel, 0, len(finishes))
	for _, finish := range finishes {
		filter := finishedOpportunitiesFilter(now)
		filter["_id"] = finish.IDOpportunity

		update := bson.M{
			"$set": bson.M{
				"id_status":       cmm.OpportunityStatusFinished.ID,
				"tag_status":      cmm.OpportunityStatusFinished.Tag,
				"status":          cmm.OpportunityStatusFinished.Name,
				"price_achieved":  finish.PriceAchieved,
				"last_updated_at": now,
			},
		}

		writes = append(writes, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
	}

	result, err := models.OpportunitiesModel.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, cmm.NewErrorHandler(err, "Error finalizando las oportunidades.", cmm.LevelDatabase, "SOPRE050")
	}

	return result.ModifiedCount, cmm.NewEmptyErrorHandler()
}

// UpdateCondition actualiza la condición
func UpdateCondition(ctx context.Context, idCondition primitive.ObjectID, newIDStatus primitive.ObjectID, newTagStatus string, newStatus string) *cmm.ErrorHandler {
	filter := bson.M{"_id": idCondition}

	// Update
	update := bson.M{
		"$set": bson.M{
			"status":          newStatus,
			"last_updated_at": time.Now().UTC(),
		},
	}

	// Usa el ctx
	_, err := models.ConditionsModel.UpdateOne(ctx, filter, update)
	// Check error
	if err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando las oportunidades.", cmm.LevelDatabase, "SOPRE005")
	}

	return cmm.NewEmptyErrorHandler()
}

// --- CSV de Google Trends por asset ---

// Formatos de fecha de la primera columna del CSV de Google Trends (por hora, por dia/semana y por mes)
var googleTrendsDateLayouts = []string{"2006-01-02T15", "2006-01-02", "2006-01"}

// ParseGoogleTrendsCSVService convierte el CSV exportado de Google Trends en una serie.
// Se ignoran las lineas previas a los datos (categoria y encabezado). Si el ultimo periodo no
// habia terminado en uploadedAt se descarta porque Google lo muestra incompleto
func ParseGoogleTrendsCSVService(content []byte, uploadedAt time.Time) (providers.GoogleTrendsFileSeries, *cmm.ErrorHandler) {
	series := providers.GoogleTrendsFileSeries{UploadedAt: uploadedAt.UTC()}

	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return series, cmm.NewErrorHandler(err, "El archivo no es un CSV valido", cmm.LevelError, "SOPRE006")
	}

	var header []string
	for _, record := range records {
		date, isDate := parseGoogleTrendsDate(record[0])
		if !isDate {
			// Las filas antes de los datos son la categoria y el encabezado, despues de los datos no se esperan mas filas
			if len(series.Points) > 0 {
				return series, cmm.NewErrorHandler(fmt.Errorf("row %q", strings.Join(record, ",")), "El CSV tiene filas invalidas despues de los datos", cmm.LevelError, "SOPRE007")
			}
			header = record
			continue
		}

		// Solo se acepta una busqueda por archivo, una comparacion de varias no tiene un unico valor por fecha
		if len(record) != 2 {
			return series, cmm.NewErrorHandler(fmt.Errorf("got %d columns", len(record)), "El CSV debe tener una sola busqueda (fecha y valor)", cmm.LevelError, "SOPRE008")
		}

		value, err := parseGoogleTrendsCSVValue(record[1])
		if !err.IsEmtpy() {
			return series, err
		}

		if len(series.Points) > 0 && !date.After(series.Points[len(series.Points)-1].Date) {
			return series, cmm.NewErrorHandler(fmt.Errorf("date %s", record[0]), "Las fechas del CSV deben estar en orden ascendente", cmm.LevelError, "SOPRE009")
		}

		series.Points = append(series.Points, providers.GoogleTrendsFilePoint{Date: date, Value: value})
	}

	if len(series.Points) < 2 {
		return series, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(series.Points)), "El CSV debe tener al menos 2 fechas", cmm.LevelError, "SOPRE010")
	}

	// El encabezado de la busqueda tiene el formato "<keyword>: (<region>)"
	if len(header) == 2 {
		keyword, _, _ := strings.Cut(header[1], ": (")
		series.Keyword = strings.TrimSpace(keyword)
	}

	// El periodo se calcula con las dos ultimas fechas y se descarta el ultimo punto si no habia terminado
	last := series.Points[len(series.Points)-1].Date
	period := last.Sub(series.Points[len(series.Points)-2].Date)
	series.PeriodSeconds = int64(period / time.Second)
	if last.Add(period).After(series.UploadedAt) {
		series.Points = series.Points[:len(series.Points)-1]
	}

	if len(series.Points) < 2 {
		return series, cmm.NewErrorHandler(fmt.Errorf("got %d complete points", len(series.Points)), "El CSV debe tener al menos 2 periodos completos", cmm.LevelError, "SOPRE011")
	}

	return series, cmm.NewEmptyErrorHandler()
}

// SaveGoogleTrendsFileService guarda el CSV del asset reemplazando el anterior. Se escribe en un archivo
// temporal y se renombra para que una lectura concurrente nunca vea un archivo a medias
func SaveGoogleTrendsFileService(idAsset primitive.ObjectID, content []byte) *cmm.ErrorHandler {
	dir := googleTrendsDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return cmm.NewErrorHandler(err, "Error creando la carpeta de Google Trends", cmm.LevelFatal, "SOPRE012")
	}

	tmp, err := os.CreateTemp(dir, idAsset.Hex()+"-*.tmp")
	if err != nil {
		return cmm.NewErrorHandler(err, "Error creando el archivo temporal", cmm.LevelFatal, "SOPRE013")
	}
	defer os.Remove(tmp.Name())

	_, writeErr := tmp.Write(content)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return cmm.NewErrorHandler(err, "Error escribiendo el archivo temporal", cmm.LevelFatal, "SOPRE014")
	}

	if err := os.Rename(tmp.Name(), googleTrendsFilePath(idAsset)); err != nil {
		return cmm.NewErrorHandler(err, "Error reemplazando el CSV del asset", cmm.LevelFatal, "SOPRE015")
	}

	return cmm.NewEmptyErrorHandler()
}

// GetGoogleTrendsFileService lee y parsea el CSV del asset. La fecha de subida es la fecha de modificacion del archivo
func GetGoogleTrendsFileService(idAsset primitive.ObjectID) (providers.GoogleTrendsFileSeries, *cmm.ErrorHandler) {
	path := googleTrendsFilePath(idAsset)

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return providers.GoogleTrendsFileSeries{}, cmm.NewErrorHandler(err, "El asset no tiene un CSV de Google Trends", cmm.LevelError, "SOPRE016")
	}
	if err != nil {
		return providers.GoogleTrendsFileSeries{}, cmm.NewErrorHandler(err, "Error leyendo el CSV del asset", cmm.LevelFatal, "SOPRE017")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return providers.GoogleTrendsFileSeries{}, cmm.NewErrorHandler(err, "Error leyendo el CSV del asset", cmm.LevelFatal, "SOPRE018")
	}

	series, parseErr := ParseGoogleTrendsCSVService(content, info.ModTime())
	if !parseErr.IsEmtpy() {
		return series, parseErr
	}

	series.IDAsset = idAsset.Hex()
	return series, cmm.NewEmptyErrorHandler()
}

// parseGoogleTrendsDate intenta leer la fecha con los formatos de Google Trends
func parseGoogleTrendsDate(text string) (time.Time, bool) {
	for _, layout := range googleTrendsDateLayouts {
		if date, err := time.Parse(layout, strings.TrimSpace(text)); err == nil {
			return date, true
		}
	}
	return time.Time{}, false
}

// parseGoogleTrendsCSVValue convierte el valor del CSV. Google escribe "<1" cuando el interes es casi nulo
func parseGoogleTrendsCSVValue(text string) (float64, *cmm.ErrorHandler) {
	text = strings.TrimSpace(text)
	if text == "<1" {
		return 0, cmm.NewEmptyErrorHandler()
	}

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, cmm.NewErrorHandler(err, fmt.Sprintf("Valor invalido en el CSV: %q", text), cmm.LevelError, "SOPRE019")
	}

	return value, cmm.NewEmptyErrorHandler()
}

// googleTrendsDir carpeta donde se guardan los CSV
func googleTrendsDir() string {
	if dir := os.Getenv(cmm.GoogleTrendsDirEnv); dir != "" {
		return dir
	}
	return cmm.GoogleTrendsDefaultDir
}

// googleTrendsFilePath ruta del CSV del asset. El nombre sale del ObjectID, nunca del nombre subido
func googleTrendsFilePath(idAsset primitive.ObjectID) string {
	return filepath.Join(googleTrendsDir(), idAsset.Hex()+".csv")
}

// CreateTriggerService inserta un trigger nuevo y retorna su id
func CreateTriggerService(trigger models.TriggersModelType) (string, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	result, err := models.TriggersModel.InsertOne(ctx, trigger)
	if err != nil {
		// Si existe un indice unico sobre el tag, dos peticiones simultaneas con el mismo tag terminan aqui
		if mongo.IsDuplicateKeyError(err) {
			return "", cmm.NewErrorHandler(err, "Ya existe un trigger con el tag enviado.", cmm.LevelError, "SOPRE022")
		}

		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SOPRE023")
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "SOPRE024")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}

// ConditionStatusChange nuevo status de una condicion que debe replicarse en las oportunidades
type ConditionStatusChange struct {
	IDCondition primitive.ObjectID
	NewStatus   cmm.CatalogStatus
}

// UpdateOpportunitiesConditionsStatusService replica el status de las condiciones en las oportunidades en curso.
// Recibe el ctx de la transaccion (SessionContext)
func UpdateOpportunitiesConditionsStatusService(ctx context.Context, changes []ConditionStatusChange) *cmm.ErrorHandler {
	if len(changes) == 0 {
		return cmm.NewEmptyErrorHandler()
	}

	now := time.Now().UTC()
	writes := make([]mongo.WriteModel, 0, len(changes))

	for _, c := range changes {
		filter := bson.M{
			"tag_status":              cmm.OpportunityStatusOngoing.Tag,
			"logical_delete":          false,
			"conditions.id_condition": c.IDCondition,
		}
		update := bson.M{
			"$set": bson.M{
				"conditions.$[cond].id_status":  c.NewStatus.ID,
				"conditions.$[cond].tag_status": c.NewStatus.Tag,
				"conditions.$[cond].status":     c.NewStatus.Name,
				"last_updated_at":               now,
			},
		}
		arrayFilters := options.ArrayFilters{Filters: []interface{}{bson.M{"cond.id_condition": c.IDCondition}}}

		writes = append(writes, mongo.NewUpdateManyModel().
			SetFilter(filter).
			SetUpdate(update).
			SetArrayFilters(arrayFilters))
	}

	if _, err := models.OpportunitiesModel.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false)); err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando las condiciones de las oportunidades.", cmm.LevelDatabase, "SOPRE025")
	}

	return cmm.NewEmptyErrorHandler()
}

// completedOpportunitiesFilter oportunidades en curso, no expiradas, que tengan al menos una condicion
// y todas sus condiciones activas
func completedOpportunitiesFilter(now time.Time) bson.M {
	return bson.M{
		"tag_status":      cmm.OpportunityStatusOngoing.Tag,
		"logical_delete":  false,
		"expiration_date": bson.M{"$gt": now},
		"conditions.0":    bson.M{"$exists": true},
		"conditions": bson.M{
			"$not": bson.M{
				"$elemMatch": bson.M{"tag_status": bson.M{"$ne": cmm.GenericStatusActive.Tag}},
			},
		},
	}
}

// OpportunityActivation oportunidad que pasa a ACTIVE con el precio de mercado de su asset (nil si no se obtuvo)
type OpportunityActivation struct {
	IDOpportunity          primitive.ObjectID
	PriceActiveOpportunity *float64
}

// ActivateOpportunitiesService cambia a ACTIVE las oportunidades y guarda su precio de activacion. Cada oportunidad
// se vuelve a filtrar con completedOpportunitiesFilter, asi no se activa una que cambio desde que se listo
func ActivateOpportunitiesService(activations []OpportunityActivation, now time.Time) (int64, *cmm.ErrorHandler) {
	if len(activations) == 0 {
		return 0, cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	writes := make([]mongo.WriteModel, 0, len(activations))
	for _, activation := range activations {
		filter := completedOpportunitiesFilter(now)
		filter["_id"] = activation.IDOpportunity

		update := bson.M{
			"$set": bson.M{
				"id_status":                cmm.OpportunityStatusActive.ID,
				"tag_status":               cmm.OpportunityStatusActive.Tag,
				"status":                   cmm.OpportunityStatusActive.Name,
				"price_active_opportunity": activation.PriceActiveOpportunity,
				"last_updated_at":          now,
			},
		}

		writes = append(writes, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
	}

	result, err := models.OpportunitiesModel.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, cmm.NewErrorHandler(err, "Error activando las oportunidades.", cmm.LevelDatabase, "SOPRE026")
	}

	return result.ModifiedCount, cmm.NewEmptyErrorHandler()
}

// ReplaceTriggerService reemplaza un trigger en una transaccion: borra logicamente el anterior (STATUS-INACTIVE) e
// inserta el nuevo, que ya trae id_reference e id_reference_root. Retorna el id del trigger nuevo.
// Requiere que MongoDB sea un replica set
func ReplaceTriggerService(idPrevious primitive.ObjectID, trigger models.TriggersModelType) (string, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	// Usamos cmm.Database porque cmm.AppConfig puede ser nil al iniciar
	session, err := cmm.Database.Client().StartSession()
	if err != nil {
		return "", cmm.NewErrorHandler(err, "Error iniciando la sesion de MongoDB.", cmm.LevelDatabase, "SOPRE042")
	}
	defer session.EndSession(ctx)

	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())

	// El callback del driver debe retornar error nil para hacer COMMIT, cualquier otro valor hace ROLLBACK
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// A. Borramos el anterior. Si ya estaba borrado otra peticion lo reemplazo primero
		filter := bson.M{"_id": idPrevious, "logical_delete": false}
		update := bson.M{
			"$set": bson.M{
				"logical_delete":  true,
				"id_status":       cmm.GenericStatusInactive.ID,
				"tag_status":      cmm.GenericStatusInactive.Tag,
				"status":          cmm.GenericStatusInactive.Name,
				"last_updated_at": time.Now().UTC(),
			},
		}

		result, errUpdate := models.TriggersModel.UpdateOne(sessCtx, filter, update)
		if errUpdate != nil {
			return nil, cmm.NewErrorHandler(errUpdate, "Error borrando el trigger anterior.", cmm.LevelDatabase, "SOPRE043")
		}
		if result.MatchedCount == 0 {
			return nil, cmm.NewErrorHandler(fmt.Errorf("trigger %s", idPrevious.Hex()), "El trigger ya fue actualizado o borrado.", cmm.LevelError, "SOPRE044")
		}

		// B. Insertamos el nuevo
		if _, errInsert := models.TriggersModel.InsertOne(sessCtx, trigger); errInsert != nil {
			if mongo.IsDuplicateKeyError(errInsert) {
				return nil, cmm.NewErrorHandler(errInsert, "Ya existe un trigger con el tag enviado.", cmm.LevelError, "SOPRE045")
			}

			return nil, cmm.NewErrorHandler(errInsert, errInsert.Error(), cmm.LevelDatabase, "SOPRE046")
		}

		return nil, nil
	}, txnOpts)
	if err != nil {
		// Conservamos el tracking code del paso que fallo dentro de la transaccion
		var handler *cmm.ErrorHandler
		if errors.As(err, &handler) {
			return "", handler
		}

		return "", cmm.NewErrorHandler(err, "Transaccion abortada reemplazando el trigger.", cmm.LevelDatabase, "SOPRE047")
	}

	return trigger.ID.Hex(), cmm.NewEmptyErrorHandler()
}

// Metodos HTTP permitidos para las APIs de las condiciones
var allowedConditionMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// BuildConditionHTTPRequestService construye la peticion de una condicion a partir de la API (plantilla generica)
// y los valores de los parametros enviados. Valida que los parametros existan en la API, que esten los requeridos,
// que respeten su tipo de dato y sus opciones STATIC, y que no reemplacen la data fija de la API.
// Los valores se colocan en query_data o body_data segun el data_location de la API
func BuildConditionHTTPRequestService(api models.ApisModelType, parameters map[string]string) (providers.HTTPRequestConfig, *cmm.ErrorHandler) {
	var config providers.HTTPRequestConfig

	// 1. Metodo y URL de la API
	method := strings.ToUpper(api.Method)
	if !slices.Contains(allowedConditionMethods, method) {
		return config, cmm.NewErrorHandler(fmt.Errorf("method %q", api.Method), fmt.Sprintf("La API tiene un metodo HTTP no soportado: %q", api.Method), cmm.LevelError, "SOPRE027")
	}

	parsedURL, err := url.Parse(api.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return config, cmm.NewErrorHandler(fmt.Errorf("url %q", api.URL), "La API no tiene una URL http o https valida", cmm.LevelError, "SOPRE028")
	}

	// 2. Parametros enviados que la API no acepta
	unknown := []string{}
	for name := range parameters {
		if !slices.ContainsFunc(api.Parameters, func(p models.ApiParameter) bool { return p.Name == name }) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return config, cmm.NewErrorHandler(fmt.Errorf("unknown parameters %v", unknown), fmt.Sprintf("La API no acepta los parametros: %s", strings.Join(unknown, ", ")), cmm.LevelError, "SOPRE029")
	}

	// 3. Data fija de la API, los parametros se agregan sobre ella
	data, errData := fixedDataToMap(api.Data)
	if !errData.IsEmtpy() {
		return config, errData
	}

	for _, parameter := range api.Parameters {
		value, exists := parameters[parameter.Name]
		if !exists && parameter.DefaultValue != nil {
			value, exists = *parameter.DefaultValue, true
		}
		if !exists {
			if parameter.IsRequired {
				return config, cmm.NewErrorHandler(fmt.Errorf("missing parameter %q", parameter.Name), fmt.Sprintf("El parametro %q es requerido", parameter.Name), cmm.LevelError, "SOPRE030")
			}
			continue
		}

		if _, fixed := data[parameter.Name]; fixed {
			return config, cmm.NewErrorHandler(fmt.Errorf("parameter %q is fixed data", parameter.Name), fmt.Sprintf("El parametro %q no puede reemplazar la data fija de la API", parameter.Name), cmm.LevelError, "SOPRE031")
		}

		typed, errValue := parseParameterValue(parameter, value)
		if !errValue.IsEmtpy() {
			return config, errValue
		}
		data[parameter.Name] = typed
	}

	// 4. Ubicamos la data segun el data_location. Una API sin data no necesita data_location
	location := strings.ToUpper(api.DataLocation)
	if len(data) > 0 {
		switch location {
		case cmm.ApiDataLocationQuery:
			query, errQuery := toQueryData(data)
			if !errQuery.IsEmtpy() {
				return config, errQuery
			}
			config.QueryData = query
		case cmm.ApiDataLocationBody:
			// ExecuteRequest no envia body en peticiones GET
			if method == http.MethodGet {
				return config, cmm.NewErrorHandler(nil, "Una API GET no puede enviar sus parametros por BODY", cmm.LevelError, "SOPRE032")
			}
			config.BodyData = data
		default:
			return config, cmm.NewErrorHandler(fmt.Errorf("data_location %q", api.DataLocation), "El data_location de la API debe ser QUERY o BODY", cmm.LevelError, "SOPRE033")
		}
	}

	// 5. Configuracion de la peticion de la API. Se copian los punteros para no compartir memoria con la API
	config.URL = api.URL
	config.Method = method
	config.DataLocation = location
	config.Headers = maps.Clone(api.Headers)
	config.TimeoutSeconds = api.TimeoutSeconds
	if api.Auth != nil {
		auth := *api.Auth
		auth.Value = "" // El secreto nunca se guarda, sale del secret_ref del proveedor
		config.Auth = &auth
	}
	if api.RetryPolicy != nil {
		retryPolicy := *api.RetryPolicy
		config.RetryPolicy = &retryPolicy
	}

	return config, cmm.NewEmptyErrorHandler()
}

// BuildConditionCallbackParametersService valida los parametros enviados para el callback: que el callback los acepte,
// que esten los requeridos (o tengan valor por defecto) y que respeten su tipo de dato. Retorna los valores tipados
// que se guardan en parameters_callback, {} si el callback no tiene parametros
func BuildConditionCallbackParametersService(callback providers.CallbackDefinition, parameters map[string]string) (map[string]any, *cmm.ErrorHandler) {
	result := map[string]any{}

	// 1. Parametros enviados que el callback no acepta
	unknown := []string{}
	for name := range parameters {
		if !slices.ContainsFunc(callback.Parameters, func(p providers.CallbackParameter) bool { return p.Name == name }) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return result, cmm.NewErrorHandler(fmt.Errorf("unknown callback parameters %v", unknown), fmt.Sprintf("El callback no acepta los parametros: %s", strings.Join(unknown, ", ")), cmm.LevelError, "SOPRE051")
	}

	// 2. Valores tipados, con el valor por defecto si no se envio
	for _, parameter := range callback.Parameters {
		value, exists := parameters[parameter.Name]
		if !exists && parameter.DefaultValue != nil {
			value, exists = *parameter.DefaultValue, true
		}
		if !exists {
			if parameter.IsRequired {
				return result, cmm.NewErrorHandler(fmt.Errorf("missing callback parameter %q", parameter.Name), fmt.Sprintf("El parametro %q del callback es requerido", parameter.Name), cmm.LevelError, "SOPRE052")
			}
			continue
		}

		// Los parametros del callback no tienen opciones, se validan como un parametro libre de API
		typed, errValue := parseParameterValue(models.ApiParameter{Name: parameter.Name, DataType: parameter.DataType}, value)
		if !errValue.IsEmtpy() {
			return result, errValue
		}

		// 3. Rango del valor numerico, si el callback lo define
		errRange := validateCallbackParameterRange(parameter, typed)
		if !errRange.IsEmtpy() {
			return result, errRange
		}
		result[parameter.Name] = typed
	}

	return result, cmm.NewEmptyErrorHandler()
}

// validateCallbackParameterRange valida que el valor ya tipado (int64 o float64) este dentro del rango inclusivo
// [min_value, max_value] del parametro. Los limites nil no se validan y los valores no numericos no tienen rango
func validateCallbackParameterRange(parameter providers.CallbackParameter, typed any) *cmm.ErrorHandler {
	var number float64
	switch value := typed.(type) {
	case float64:
		number = value
	case int64:
		number = float64(value)
	default:
		return cmm.NewEmptyErrorHandler()
	}

	belowMin := parameter.MinValue != nil && number < *parameter.MinValue
	aboveMax := parameter.MaxValue != nil && number > *parameter.MaxValue
	if belowMin || aboveMax {
		rangeText := callbackParameterRangeText(parameter)
		return cmm.NewErrorHandler(fmt.Errorf("parameter %q value %v not %s", parameter.Name, number, rangeText), fmt.Sprintf("El parametro %q del callback esta fuera del rango permitido (%s)", parameter.Name, rangeText), cmm.LevelError, "SOPRE053")
	}

	return cmm.NewEmptyErrorHandler()
}

// callbackParameterRangeText texto del rango del parametro para el mensaje de error (ej: "entre 0 y 100" o "mayor o igual a 0")
func callbackParameterRangeText(parameter providers.CallbackParameter) string {
	switch {
	case parameter.MinValue != nil && parameter.MaxValue != nil:
		return fmt.Sprintf("entre %v y %v", *parameter.MinValue, *parameter.MaxValue)
	case parameter.MinValue != nil:
		return fmt.Sprintf("mayor o igual a %v", *parameter.MinValue)
	default:
		return fmt.Sprintf("menor o igual a %v", *parameter.MaxValue)
	}
}

// parseParameterValue valida el valor segun el tipo de dato y las opciones del parametro. Retorna el valor con su tipo
// para que el body se envie con tipos JSON (5 y no "5")
func parseParameterValue(parameter models.ApiParameter, value string) (any, *cmm.ErrorHandler) {
	if parameter.Options != nil {
		switch parameter.Options.Source {
		case cmm.ApiParameterOptionsStatic:
			if !slices.ContainsFunc(parameter.Options.Values, func(o models.ApiParameterOption) bool { return o.Value == value }) {
				return nil, cmm.NewErrorHandler(fmt.Errorf("value %q not in options", value), fmt.Sprintf("El valor %q no es una opcion valida del parametro %q", value, parameter.Name), cmm.LevelError, "SOPRE034")
			}
		case cmm.ApiParameterOptionsApi:
			// TODO: las opciones vienen de otra API y no se consultan al crear la condicion
		default:
			return nil, cmm.NewErrorHandler(fmt.Errorf("options source %q", parameter.Options.Source), fmt.Sprintf("El parametro %q tiene un origen de opciones no soportado", parameter.Name), cmm.LevelError, "SOPRE035")
		}
	}

	var typed any
	var err error
	switch parameter.DataType {
	case cmm.ApiParameterTypeString:
		typed = value
	case cmm.ApiParameterTypeInteger:
		typed, err = strconv.ParseInt(value, 10, 64)
	case cmm.ApiParameterTypeFloat:
		typed, err = strconv.ParseFloat(value, 64)
	case cmm.ApiParameterTypeBoolean:
		typed, err = strconv.ParseBool(value)
	case cmm.ApiParameterTypeDate:
		// La fecha se envia como texto, solo se valida el formato
		_, err = time.Parse(time.DateOnly, value)
		typed = value
	default:
		return nil, cmm.NewErrorHandler(fmt.Errorf("data_type %q", parameter.DataType), fmt.Sprintf("El parametro %q tiene un tipo de dato no soportado", parameter.Name), cmm.LevelError, "SOPRE036")
	}

	if err != nil {
		return nil, cmm.NewErrorHandler(err, fmt.Sprintf("El valor %q del parametro %q no es de tipo %s", value, parameter.Name, parameter.DataType), cmm.LevelError, "SOPRE037")
	}

	return typed, cmm.NewEmptyErrorHandler()
}

// fixedDataToMap convierte la data fija de la API (leida de Mongo como interface{}) en un mapa nuevo
func fixedDataToMap(data any) (map[string]any, *cmm.ErrorHandler) {
	result := map[string]any{}

	switch value := data.(type) {
	case nil:
	case primitive.D:
		for _, element := range value {
			result[element.Key] = element.Value
		}
	case primitive.M:
		maps.Copy(result, value)
	case map[string]any:
		maps.Copy(result, value)
	case map[string]string:
		for key, val := range value {
			result[key] = val
		}
	default:
		return nil, cmm.NewErrorHandler(fmt.Errorf("data type %T", data), "La data fija de la API debe ser un objeto", cmm.LevelError, "SOPRE038")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// toQueryData convierte la data a texto para enviarla por query. Solo acepta valores simples
func toQueryData(data map[string]any) (map[string]string, *cmm.ErrorHandler) {
	query := make(map[string]string, len(data))

	for key, value := range data {
		switch v := value.(type) {
		case string:
			query[key] = v
		case bool:
			query[key] = strconv.FormatBool(v)
		case int:
			query[key] = strconv.Itoa(v)
		case int32:
			query[key] = strconv.FormatInt(int64(v), 10)
		case int64:
			query[key] = strconv.FormatInt(v, 10)
		case float64:
			query[key] = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			return nil, cmm.NewErrorHandler(fmt.Errorf("query value %q of type %T", key, value), fmt.Sprintf("El valor de %q no se puede enviar por QUERY", key), cmm.LevelError, "SOPRE039")
		}
	}

	return query, cmm.NewEmptyErrorHandler()
}

// ---------- Combinaciones imposibles de condiciones: el trigger nunca pasaria a ACTIVE ----------

// exclusiveConditionZones callbacks que leen la misma serie y son excluyentes entre si: mismo grupo y zona distinta
var exclusiveConditionZones = map[string][2]string{
	"FearAndGreedExtremeFear":  {"FEAR_AND_GREED", "EXTREME_FEAR"},
	"FearAndGreedFear":         {"FEAR_AND_GREED", "FEAR"},
	"FearAndGreedNeutral":      {"FEAR_AND_GREED", "NEUTRAL"},
	"FearAndGreedGreed":        {"FEAR_AND_GREED", "GREED"},
	"FearAndGreedExtremeGreed": {"FEAR_AND_GREED", "EXTREME_GREED"},
	"FedNextMeetingCutLikely":  {"FED_NEXT_MEETING", "CUT"},
	"FedNextMeetingHoldLikely": {"FED_NEXT_MEETING", "HOLD"},
	"FedNextMeetingHikeLikely": {"FED_NEXT_MEETING", "HIKE"},
	"GoogleTrendsHigh":         {"GOOGLE_TRENDS", "HIGH"},
	"GoogleTrendsLow":          {"GOOGLE_TRENDS", "LOW"},
	"GoogleTrendsFileHigh":     {"GOOGLE_TRENDS_FILE", "HIGH"},
	"GoogleTrendsFileLow":      {"GOOGLE_TRENDS_FILE", "LOW"},
	"GoogleTrendsFileNewMax":   {"GOOGLE_TRENDS_FILE_EXTREME", "MAX"},
	"GoogleTrendsFileNewMin":   {"GOOGLE_TRENDS_FILE_EXTREME", "MIN"},
	"FredSeriesRising":         {"FRED_SLOPE", "RISING"},
	"FredSeriesFalling":        {"FRED_SLOPE", "FALLING"},
}

// conditionThresholdRule callback de umbral (estricto: > o <). mirror indica que la serie es 100 - serie del grupo
// (OTHERS2 = 100 - BTC.D), asi "OTHERS2 > x" equivale a "BTC.D < 100 - x"
type conditionThresholdRule struct {
	group   string
	isAbove bool
	mirror  bool
	fixed   *float64 // umbral fijo del callback, nil si viene en parameters_callback.threshold
}

var btcDominanceFixedThreshold = 60.0

var conditionThresholdRules = map[string]conditionThresholdRule{
	"FearAndGreedAbove":     {group: "FEAR_AND_GREED", isAbove: true},
	"FearAndGreedBelow":     {group: "FEAR_AND_GREED", isAbove: false},
	"BtcDominanceAbove":     {group: "BTC_DOMINANCE", isAbove: true},
	"BtcDominanceBelow":     {group: "BTC_DOMINANCE", isAbove: false},
	"BtcDominanceAbove60":   {group: "BTC_DOMINANCE", isAbove: true, fixed: &btcDominanceFixedThreshold},
	"Others2DominanceAbove": {group: "BTC_DOMINANCE", isAbove: true, mirror: true},
	"Others2DominanceBelow": {group: "BTC_DOMINANCE", isAbove: false, mirror: true},
	"Others3DominanceAbove": {group: "OTHERS3_DOMINANCE", isAbove: true},
	"Others3DominanceBelow": {group: "OTHERS3_DOMINANCE", isAbove: false},
	"UsdtDominanceAbove":    {group: "USDT_DOMINANCE", isAbove: true},
	"UsdtDominanceBelow":    {group: "USDT_DOMINANCE", isAbove: false},
}

// conditionBound limite ya normalizado al grupo: value > limit (isAbove) o value < limit
type conditionBound struct {
	isAbove bool
	limit   float64
}

// FindIncompatibleConditionsService busca pares de condiciones de un trigger que no pueden ser true a la vez (zonas
// excluyentes de la misma serie o umbrales sin rango) o que estan repetidas. Retorna un texto por par, [] si no hay
func FindIncompatibleConditionsService(conditions []models.ConditionsModelType) []string {
	problems := []string{}
	for i := 0; i < len(conditions); i++ {
		for j := i + 1; j < len(conditions); j++ {
			problem := conditionPairProblem(conditions[i], conditions[j])
			if problem != "" {
				problems = append(problems, fmt.Sprintf("%s (%s) y %s (%s): %s", conditions[i].Tag, conditions[i].ID.Hex(), conditions[j].Tag, conditions[j].ID.Hex(), problem))
			}
		}
	}
	return problems
}

func conditionPairProblem(a, b models.ConditionsModelType) string {
	if a.CallbackFunction == b.CallbackFunction && sameConditionSource(a, b) && reflect.DeepEqual(a.ParametersCallback, b.ParametersCallback) {
		return "son la misma condicion"
	}

	zoneA, okA := exclusiveConditionZones[a.CallbackFunction]
	zoneB, okB := exclusiveConditionZones[b.CallbackFunction]
	if okA && okB && zoneA[0] == zoneB[0] && zoneA[1] != zoneB[1] && sameConditionSource(a, b) {
		return "son excluyentes, nunca se cumplen a la vez"
	}

	// Las dominancias salen de la misma respuesta (CoinMarketCap global) aunque el callback sea distinto
	groupA, boundA, okA := conditionThresholdBound(a)
	groupB, boundB, okB := conditionThresholdBound(b)
	if !okA || !okB || groupA != groupB || !sameConditionSource(a, b) || boundA.isAbove == boundB.isAbove {
		return ""
	}
	above, below := boundA, boundB
	if !boundA.isAbove {
		above, below = boundB, boundA
	}
	if below.limit <= above.limit {
		return fmt.Sprintf("piden un valor > %v y < %v a la vez", above.limit, below.limit)
	}
	return ""
}

// sameConditionSource dos condiciones leen la misma serie si piden lo mismo a la misma API
func sameConditionSource(a, b models.ConditionsModelType) bool {
	if a.IDApi != b.IDApi {
		return false
	}
	ra, rb := a.HttpRequest, b.HttpRequest
	return ra.URL == rb.URL && ra.Method == rb.Method &&
		reflect.DeepEqual(ra.QueryData, rb.QueryData) && reflect.DeepEqual(ra.BodyData, rb.BodyData)
}

func conditionThresholdBound(c models.ConditionsModelType) (string, conditionBound, bool) {
	rule, exists := conditionThresholdRules[c.CallbackFunction]
	if !exists {
		return "", conditionBound{}, false
	}

	threshold, ok := 0.0, true
	if rule.fixed != nil {
		threshold = *rule.fixed
	} else {
		threshold, ok = numberToFloat(c.ParametersCallback["threshold"])
	}
	if !ok {
		return "", conditionBound{}, false
	}

	bound := conditionBound{isAbove: rule.isAbove, limit: threshold}
	if rule.mirror {
		bound = conditionBound{isAbove: !rule.isAbove, limit: 100 - threshold}
	}
	return rule.group, bound, true
}

// numberToFloat convierte los numeros que guarda parameters_callback (int64/float64 al crear, int32 al leer de Mongo)
func numberToFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	}
	return 0, false
}

// ---------- Agentes: cola de tesis, corridas y revision ----------

// CreateAgentThesisService inserta una tesis en la cola de los agentes
func CreateAgentThesisService(thesis models.AgentThesesModelType) (string, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	result, err := models.AgentThesesModel.InsertOne(ctx, thesis)
	if err != nil {
		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SOPRE054")
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "SOPRE055")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}

// DeleteAgentThesisService borra logicamente una tesis que sigue pendiente
func DeleteAgentThesisService(idThesis primitive.ObjectID, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idThesis, "tag_status": cmm.AgentThesisStatusPending.Tag, "logical_delete": false}
	update := bson.M{"$set": bson.M{"logical_delete": true, "last_updated_at": now}}

	result, err := models.AgentThesesModel.UpdateOne(ctx, filter, update)
	if err != nil {
		return cmm.NewErrorHandler(err, "Error borrando la tesis.", cmm.LevelDatabase, "SOPRE056")
	}
	if result.MatchedCount == 0 {
		return cmm.NewErrorHandler(fmt.Errorf("thesis %s", idThesis.Hex()), "La tesis ya fue tomada por una corrida o borrada.", cmm.LevelError, "SOPRE057")
	}

	return cmm.NewEmptyErrorHandler()
}

// ClaimAgentThesesService toma hasta AgentThesesPerRun tesis, las mas antiguas primero (FIFO), y las pasa a PROCESSING.
// Tambien retoma las que quedaron en PROCESSING por mas de AgentThesisClaimTimeout (corridas que fallaron sin cerrar).
// Cada tesis se toma con un FindOneAndUpdate atomico para que dos corridas no procesen la misma
func ClaimAgentThesesService(now time.Time) ([]models.AgentThesesModelType, *cmm.ErrorHandler) {
	claimed := make([]models.AgentThesesModelType, 0, cmm.AgentThesesPerRun)

	filter := bson.M{
		"logical_delete": false,
		"$or": []bson.M{
			{"tag_status": cmm.AgentThesisStatusPending.Tag},
			{"tag_status": cmm.AgentThesisStatusProcessing.Tag, "claimed_at": bson.M{"$lt": now.Add(-cmm.AgentThesisClaimTimeout)}},
		},
	}
	update := bson.M{"$set": bson.M{
		"id_status":       cmm.AgentThesisStatusProcessing.ID,
		"tag_status":      cmm.AgentThesisStatusProcessing.Tag,
		"status":          cmm.AgentThesisStatusProcessing.Name,
		"claimed_at":      now,
		"last_updated_at": now,
	}}
	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "created_date", Value: 1}, {Key: "_id", Value: 1}}).
		SetReturnDocument(options.After)

	for len(claimed) < cmm.AgentThesesPerRun {
		ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
		var thesis models.AgentThesesModelType
		err := models.AgentThesesModel.FindOneAndUpdate(ctx, filter, update, opts).Decode(&thesis)
		cancel()
		if err == mongo.ErrNoDocuments {
			break
		}
		if err != nil {
			return claimed, cmm.NewErrorHandler(err, "Error tomando las tesis de la cola.", cmm.LevelDatabase, "SOPRE058")
		}
		claimed = append(claimed, thesis)
	}

	return claimed, cmm.NewEmptyErrorHandler()
}

// CreateAgentRunService inserta la corrida de una tesis
func CreateAgentRunService(run models.AgentRunsModelType) (string, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	result, err := models.AgentRunsModel.InsertOne(ctx, run)
	if err != nil {
		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SOPRE059")
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "SOPRE060")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}

// AgentRunFinish resultado con el que se cierra una corrida
type AgentRunFinish struct {
	IDRun        primitive.ObjectID
	IDThesis     primitive.ObjectID
	Status       cmm.CatalogStatus
	Steps        []models.AgentRunStep
	IDConditions []primitive.ObjectID
	IDTrigger    *primitive.ObjectID
	Error        *string
	// La tesis vuelve a la cola (PENDING) en vez de quedar PROCESSED, ej: se acabo el uso del plan de Claude
	Requeue bool
}

// FinishAgentRunService cierra la corrida y su tesis en una transaccion
func FinishAgentRunService(finish AgentRunFinish, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	// Usamos cmm.Database porque cmm.AppConfig puede ser nil al iniciar
	session, err := cmm.Database.Client().StartSession()
	if err != nil {
		return cmm.NewErrorHandler(err, "Error iniciando la sesion de MongoDB.", cmm.LevelDatabase, "SOPRE061")
	}
	defer session.EndSession(ctx)

	thesisStatus := cmm.AgentThesisStatusProcessed
	if finish.Requeue {
		thesisStatus = cmm.AgentThesisStatusPending
	}

	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// A. Cerramos la corrida si sigue en ejecucion
		runFilter := bson.M{"_id": finish.IDRun, "tag_status": cmm.AgentRunStatusRunning.Tag, "logical_delete": false}
		runUpdate := bson.M{"$set": bson.M{
			"id_status":       finish.Status.ID,
			"tag_status":      finish.Status.Tag,
			"status":          finish.Status.Name,
			"steps":           finish.Steps,
			"id_conditions":   finish.IDConditions,
			"id_trigger":      finish.IDTrigger,
			"error":           finish.Error,
			"finished_at":     now,
			"last_updated_at": now,
		}}
		result, errUpdate := models.AgentRunsModel.UpdateOne(sessCtx, runFilter, runUpdate)
		if errUpdate != nil {
			return nil, cmm.NewErrorHandler(errUpdate, "Error cerrando la corrida.", cmm.LevelDatabase, "SOPRE062")
		}
		if result.MatchedCount == 0 {
			return nil, cmm.NewErrorHandler(fmt.Errorf("run %s", finish.IDRun.Hex()), "La corrida ya fue cerrada.", cmm.LevelError, "SOPRE063")
		}

		// B. La tesis queda procesada o vuelve a la cola. Si otra corrida ya la retomo no se toca
		thesisFilter := bson.M{"_id": finish.IDThesis, "tag_status": cmm.AgentThesisStatusProcessing.Tag, "logical_delete": false}
		thesisSet := bson.M{
			"id_status":       thesisStatus.ID,
			"tag_status":      thesisStatus.Tag,
			"status":          thesisStatus.Name,
			"last_updated_at": now,
		}
		if finish.Requeue {
			thesisSet["claimed_at"] = nil
		}
		if _, errThesis := models.AgentThesesModel.UpdateOne(sessCtx, thesisFilter, bson.M{"$set": thesisSet}); errThesis != nil {
			return nil, cmm.NewErrorHandler(errThesis, "Error actualizando la tesis.", cmm.LevelDatabase, "SOPRE064")
		}

		return nil, nil
	}, txnOpts)
	if err != nil {
		var handler *cmm.ErrorHandler
		if errors.As(err, &handler) {
			return handler
		}
		return cmm.NewErrorHandler(err, "Error en la transaccion de la corrida.", cmm.LevelDatabase, "SOPRE065")
	}

	return cmm.NewEmptyErrorHandler()
}

// reviewSet campos que cambia la revision. Aprobado queda con el status approvedStatus; rechazado queda NOT-APPROVED
// (sin borrado logico, se conserva para el historial de los agentes)
func reviewSet(approve bool, approvedStatus cmm.CatalogStatus, now time.Time) bson.M {
	status := approvedStatus
	if !approve {
		status = cmm.GenericStatusNotApproved
	}
	return bson.M{
		"id_status":       status.ID,
		"tag_status":      status.Tag,
		"status":          status.Name,
		"last_updated_at": now,
	}
}

// ReviewConditionService aprueba o rechaza una condicion pendiente de aprobacion. Aprobada pasa a inactiva (aun no se
// cumple) y el cronjob la evalua en el siguiente ciclo
func ReviewConditionService(idCondition primitive.ObjectID, approve bool, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idCondition, "tag_status": cmm.GenericStatusPendingForApprove.Tag, "logical_delete": false}
	set := reviewSet(approve, cmm.GenericStatusInactive, now)
	set["next_execution_at"] = now

	result, err := models.ConditionsModel.UpdateOne(ctx, filter, bson.M{"$set": set})
	if err != nil {
		return cmm.NewErrorHandler(err, "Error revisando la condicion.", cmm.LevelDatabase, "SOPRE066")
	}
	if result.MatchedCount == 0 {
		return cmm.NewErrorHandler(fmt.Errorf("condition %s", idCondition.Hex()), "La condicion ya fue revisada o borrada.", cmm.LevelError, "SOPRE067")
	}

	return cmm.NewEmptyErrorHandler()
}

// ReviewTriggerService aprueba o rechaza un trigger pendiente de aprobacion en una transaccion. Aprobado queda activo
// y tambien se aprueban sus condiciones pendientes (idConditions) para que pueda activarse. Rechazado queda NOT-APPROVED
func ReviewTriggerService(idTrigger primitive.ObjectID, approve bool, idConditions []primitive.ObjectID, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	// Usamos cmm.Database porque cmm.AppConfig puede ser nil al iniciar
	session, err := cmm.Database.Client().StartSession()
	if err != nil {
		return cmm.NewErrorHandler(err, "Error iniciando la sesion de MongoDB.", cmm.LevelDatabase, "SOPRE068")
	}
	defer session.EndSession(ctx)

	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// A. El trigger
		filter := bson.M{"_id": idTrigger, "tag_status": cmm.GenericStatusPendingForApprove.Tag, "logical_delete": false}
		set := reviewSet(approve, cmm.GenericStatusActive, now)
		result, errUpdate := models.TriggersModel.UpdateOne(sessCtx, filter, bson.M{"$set": set})
		if errUpdate != nil {
			return nil, cmm.NewErrorHandler(errUpdate, "Error revisando el trigger.", cmm.LevelDatabase, "SOPRE069")
		}
		if result.MatchedCount == 0 {
			return nil, cmm.NewErrorHandler(fmt.Errorf("trigger %s", idTrigger.Hex()), "El trigger ya fue revisado o borrado.", cmm.LevelError, "SOPRE070")
		}

		// B. Sus condiciones pendientes, solo al aprobar
		if !approve || len(idConditions) == 0 {
			return nil, nil
		}
		conditionsFilter := bson.M{"_id": bson.M{"$in": idConditions}, "tag_status": cmm.GenericStatusPendingForApprove.Tag, "logical_delete": false}
		conditionsSet := reviewSet(true, cmm.GenericStatusInactive, now)
		conditionsSet["next_execution_at"] = now
		if _, errConditions := models.ConditionsModel.UpdateMany(sessCtx, conditionsFilter, bson.M{"$set": conditionsSet}); errConditions != nil {
			return nil, cmm.NewErrorHandler(errConditions, "Error aprobando las condiciones del trigger.", cmm.LevelDatabase, "SOPRE071")
		}

		return nil, nil
	}, txnOpts)
	if err != nil {
		var handler *cmm.ErrorHandler
		if errors.As(err, &handler) {
			return handler
		}
		return cmm.NewErrorHandler(err, "Error en la transaccion de la revision.", cmm.LevelDatabase, "SOPRE072")
	}

	return cmm.NewEmptyErrorHandler()
}
