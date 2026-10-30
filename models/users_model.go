package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UsersModelType usuario del backoffice. Solo el username es obligatorio, los datos personales son opcionales
// y se guardan como null. Los datos de la contraseña y de la sesion no se envian al frontend (json:"-")
type UsersModelType struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Username string             `bson:"username" json:"username" description:"Nombre de usuario unico, en minusculas"`
	// Opcionales
	Email    *string `bson:"email" json:"email" description:"Correo del usuario, null si no tiene"`
	Name     *string `bson:"name" json:"name" description:"Nombre, null si no tiene"`
	LastName *string `bson:"last_name" json:"lastName" description:"Apellido, null si no tiene"`

	// Privilegio (catalogo USER_PRIVILEGES). Solo puede existir un super usuario
	IDTypePrivilege      primitive.ObjectID `bson:"id_type_privilege" json:"idTypePrivilege" description:"Id del privilegio del usuario"`
	TagPrivilege         string             `bson:"tag_privilege" json:"tagPrivilege"`
	NamePrivilege        string             `bson:"name_privilege" json:"namePrivilege"`
	DescriptionPrivilege string             `bson:"description_privilege" json:"descriptionPrivilege"`

	IsActive bool `bson:"is_active" json:"isActive" description:"Un usuario inactivo no puede iniciar sesion"`

	// Contraseña: pbkdf2_sha256$<iteraciones>$<salt>$<hash>
	PasswordHash      string    `bson:"password_hash" json:"-" description:"Hash de la contraseña con su salt e iteraciones"`
	PasswordUpdatedAt time.Time `bson:"password_updated_at" json:"-" description:"Ultimo cambio de contraseña"`

	// Login
	FailedLoginAttempts int        `bson:"failed_login_attempts" json:"-" description:"Intentos fallidos de login seguidos"`
	LockedUntil         *time.Time `bson:"locked_until" json:"-" description:"Bloqueo por intentos fallidos, null si no esta bloqueado"`
	LastLoginAt         *time.Time `bson:"last_login_at" json:"lastLoginAt" description:"Ultimo login exitoso, null si nunca inicio sesion"`

	// Limites y permisos para futuras funcionalidades. null en los maximos significa sin limite
	MaxTriggersQuantity   *int                 `bson:"max_triggers_quantity" json:"maxTriggersQuantity" description:"Maximo de triggers que puede crear, null sin limite"`
	MaxConditionsQuantity *int                 `bson:"max_conditions_quantity" json:"maxConditionsQuantity" description:"Maximo de condiciones que puede crear, null sin limite"`
	AvailableApis         []primitive.ObjectID `bson:"available_apis" json:"availableApis" description:"APIs que puede usar en sus condiciones, [] si aun no se restringe"`
	Config                map[string]any       `bson:"config" json:"config" description:"Preferencias del usuario (idioma, zona horaria, etc), {} si no tiene"`
	IDCreatedBy           *primitive.ObjectID  `bson:"id_created_by" json:"idCreatedBy" description:"Usuario que lo creo, null si se creo desde el servidor"`

	// Comunes
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos"`
}

// RecordOwnerData usuario que creo el registro con su privilegio al momento de crearlo. Se embebe en los triggers,
// las condiciones, las oportunidades y las sesiones (bson inline)
type RecordOwnerData struct {
	IDUser                   primitive.ObjectID `bson:"id_user" json:"idUser" description:"Usuario que creo el registro"`
	IDTypePrivilege          primitive.ObjectID `bson:"id_type_privilege" json:"idTypePrivilege" description:"Id del privilegio del usuario (catalogo USER_PRIVILEGES)"`
	TagTypePrivilege         string             `bson:"tag_type_privilege" json:"tagTypePrivilege"`
	NameTypePrivilege        string             `bson:"name_type_privilege" json:"nameTypePrivilege"`
	DescriptionTypePrivilege string             `bson:"description_type_privilege" json:"descriptionTypePrivilege"`
}

// Model pointer
var UsersModel *mongo.Collection

// NewRecordOwnerData copia el usuario y su privilegio al registro que crea
func NewRecordOwnerData(user UsersModelType) RecordOwnerData {
	return RecordOwnerData{
		IDUser:                   user.ID,
		IDTypePrivilege:          user.IDTypePrivilege,
		TagTypePrivilege:         user.TagPrivilege,
		NameTypePrivilege:        user.NamePrivilege,
		DescriptionTypePrivilege: user.DescriptionPrivilege,
	}
}

// Helpers

// MissingUsers retorna en hexadecimal los ids de expected que no estan en found, en el mismo orden de expected
func MissingUsers(expected []primitive.ObjectID, found []UsersModelType) []string {
	foundIDs := make(map[primitive.ObjectID]bool, len(found))
	for _, user := range found {
		foundIDs[user.ID] = true
	}

	missing := []string{}
	for _, id := range expected {
		if !foundIDs[id] {
			missing = append(missing, id.Hex())
		}
	}

	return missing
}

// UserIsLocked indica si el usuario esta bloqueado por intentos fallidos de login en el momento now
func UserIsLocked(user UsersModelType, now time.Time) bool {
	if user.LockedUntil == nil {
		return false
	}

	return now.Before(*user.LockedUntil)
}

// CONSTRUCTORS

type UserOptionalData struct {
	Email       *string
	Name        *string
	LastName    *string
	IDCreatedBy *primitive.ObjectID
}

// NewUserModel crea un usuario activo con la contraseña ya hasheada y el privilegio copiado del catalogo. Los campos
// opcionales se guardan como null, los arreglos como [] y config como {} para mantener la integridad de la data
func NewUserModel(username string, privilege CatalogsModelType, passwordHash string, opts *UserOptionalData) (UsersModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	user := UsersModelType{
		ID:                   primitive.NewObjectID(),
		Username:             username,
		IDTypePrivilege:      privilege.ID,
		TagPrivilege:         privilege.Tag,
		NamePrivilege:        privilege.Name,
		DescriptionPrivilege: privilege.Description,
		IsActive:             true,
		PasswordHash:         passwordHash,
		PasswordUpdatedAt:    now,
		FailedLoginAttempts:  0,
		AvailableApis:        []primitive.ObjectID{},
		Config:               map[string]any{},

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   now,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}

	if opts != nil {
		user.Email = opts.Email
		user.Name = opts.Name
		user.LastName = opts.LastName
		user.IDCreatedBy = opts.IDCreatedBy
	}

	return user, cmm.NewEmptyErrorHandler()
}
