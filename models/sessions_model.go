package models

import (
	"os"
	"slices"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SessionsModelType sesion abierta por un login. Solo se guarda el hash del token, el token solo lo conoce el cliente.
// El indice TTL sobre expires_at borra las sesiones vencidas. Guarda una copia del usuario y su privilegio al iniciar
// sesion para autenticar con una sola consulta: si el usuario cambia de privilegio o se desactiva hay que borrar sus sesiones
type SessionsModelType struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	TokenHash string             `bson:"token_hash" json:"-" description:"SHA-256 del token en hexadecimal"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expiresAt" description:"Vencimiento de la sesion"`

	// Usuario de la sesion (id_user) y su privilegio
	RecordOwnerData `bson:",inline"`
	Username        string `bson:"username" json:"username" description:"Username del usuario de la sesion"`

	// Comunes
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos"`
}

// Model pointer
var SessionsModel *mongo.Collection

// Helpers

// SessionHasPrivilege indica si el tag del privilegio del usuario de la sesion (catalogo USER_PRIVILEGES) esta entre
// los permitidos. Sin tags permitidos ningun usuario tiene acceso
func SessionHasPrivilege(session SessionsModelType, allowedTags []string) bool {
	return slices.Contains(allowedTags, session.TagTypePrivilege)
}

// CONSTRUCTORS

// NewSessionModel crea la sesion del usuario que vence SessionTTL despues de now, con la copia del usuario y su privilegio
func NewSessionModel(user UsersModelType, tokenHash string, now time.Time) (SessionsModelType, *cmm.ErrorHandler) {
	now = now.UTC()

	session := SessionsModelType{
		ID:              primitive.NewObjectID(),
		TokenHash:       tokenHash,
		ExpiresAt:       now.Add(cmm.SessionTTL),
		RecordOwnerData: NewRecordOwnerData(user),
		Username:        user.Username,

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   now,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}

	return session, cmm.NewEmptyErrorHandler()
}
