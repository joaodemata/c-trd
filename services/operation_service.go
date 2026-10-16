package services

import (
	"bytes"
	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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
