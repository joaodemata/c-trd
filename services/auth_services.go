package services

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	cmm "c_trd/common"
	"c_trd/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Prefijo del hash guardado en password_hash, permite cambiar de algoritmo en el futuro
const passwordHashAlgorithm = "pbkdf2_sha256"

// Tamaños en bytes del salt y del hash de la contraseña y del token de sesion
const (
	passwordSaltBytes = 16
	passwordKeyBytes  = 32
	sessionTokenBytes = 32
)

// ---------------------------------------------------------------------------
// Contraseñas
// ---------------------------------------------------------------------------

// HashPasswordService genera el hash PBKDF2-SHA256 de la contraseña con un salt aleatorio.
// Retorna pbkdf2_sha256$<iteraciones>$<salt>$<hash> en base64 sin padding
func HashPasswordService(password string) (string, *cmm.ErrorHandler) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelFatal, "SAUTE001")
	}

	key, err := pbkdf2.Key(sha256.New, password, salt, cmm.PasswordHashIterations, passwordKeyBytes)
	if err != nil {
		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelFatal, "SAUTE002")
	}

	encoded := fmt.Sprintf("%s$%d$%s$%s",
		passwordHashAlgorithm,
		cmm.PasswordHashIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)

	return encoded, cmm.NewEmptyErrorHandler()
}

// VerifyPasswordService compara la contraseña con el hash guardado en tiempo constante. Usa las iteraciones
// guardadas en el hash, asi los hashes viejos siguen funcionando si cambia PasswordHashIterations
func VerifyPasswordService(password, encoded string) (bool, *cmm.ErrorHandler) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordHashAlgorithm {
		return false, cmm.NewErrorHandler(nil, "Formato de hash de contraseña invalido", cmm.LevelFatal, "SAUTE003")
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, cmm.NewErrorHandler(err, "Iteraciones del hash de contraseña invalidas", cmm.LevelFatal, "SAUTE004")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, cmm.NewErrorHandler(err, "Salt del hash de contraseña invalido", cmm.LevelFatal, "SAUTE005")
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false, cmm.NewErrorHandler(err, "Hash de contraseña invalido", cmm.LevelFatal, "SAUTE006")
	}

	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false, cmm.NewErrorHandler(err, err.Error(), cmm.LevelFatal, "SAUTE007")
	}

	match := subtle.ConstantTimeCompare(key, expected) == 1

	return match, cmm.NewEmptyErrorHandler()
}

// ---------------------------------------------------------------------------
// Tokens de sesion
// ---------------------------------------------------------------------------

// GenerateSessionTokenService genera un token aleatorio (base64url) y su hash. El token se envia al cliente y en la
// base de datos solo se guarda el hash, asi una copia de la base de datos no sirve para iniciar sesion
func GenerateSessionTokenService() (string, string, *cmm.ErrorHandler) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelFatal, "SAUTE008")
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := HashSessionTokenService(token)

	return token, tokenHash, cmm.NewEmptyErrorHandler()
}

// HashSessionTokenService SHA-256 del token en hexadecimal, es el valor que se busca en sessions.token_hash
func HashSessionTokenService(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Base de datos: usuarios
// ---------------------------------------------------------------------------

// getUserByFilterService busca un usuario no borrado. Si no existe retorna el usuario vacio sin error
func getUserByFilterService(filter bson.M) (models.UsersModelType, *cmm.ErrorHandler) {
	var result models.UsersModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter["logical_delete"] = false

	err := models.UsersModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE009")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetUserService busca un usuario no borrado por su ID. Si no existe retorna el usuario vacio sin error
func GetUserService(idUser primitive.ObjectID) (models.UsersModelType, *cmm.ErrorHandler) {
	return getUserByFilterService(bson.M{"_id": idUser})
}

// GetUserByUsernameService busca un usuario no borrado por su username. Si no existe retorna el usuario vacio sin error
func GetUserByUsernameService(username string) (models.UsersModelType, *cmm.ErrorHandler) {
	return getUserByFilterService(bson.M{"username": username})
}

// GetUserByEmailService busca un usuario no borrado por su email. Si no existe retorna el usuario vacio sin error
func GetUserByEmailService(email string) (models.UsersModelType, *cmm.ErrorHandler) {
	return getUserByFilterService(bson.M{"email": email})
}

// CreateUserService inserta el usuario y retorna su ID
func CreateUserService(user models.UsersModelType) (string, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	result, err := models.UsersModel.InsertOne(ctx, user)
	if err != nil {
		// Los indices unicos (username, email y un solo super usuario) rechazan el insert si dos peticiones compiten
		if mongo.IsDuplicateKeyError(err) {
			return "", cmm.NewErrorHandler(err, "Ya existe un usuario con el username o email enviado.", cmm.LevelError, "SAUTE010")
		}

		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE011")
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "SAUTE012")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}

// RegisterFailedLoginService suma un intento fallido de login. Al llegar a LoginMaxFailedAttempts bloquea el
// usuario por LoginLockDuration y reinicia el contador
func RegisterFailedLoginService(idUser primitive.ObjectID, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	var updated models.UsersModelType

	filter := bson.M{"_id": idUser}
	update := bson.M{
		"$inc": bson.M{"failed_login_attempts": 1},
		"$set": bson.M{"last_updated_at": now},
	}
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetProjection(bson.M{"_id": 1, "failed_login_attempts": 1})

	err := models.UsersModel.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE013")
	}

	if updated.FailedLoginAttempts < cmm.LoginMaxFailedAttempts {
		return cmm.NewEmptyErrorHandler()
	}

	ctxLock, cancelLock := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancelLock()

	lockedUntil := now.Add(cmm.LoginLockDuration)
	lock := bson.M{"$set": bson.M{"locked_until": lockedUntil, "failed_login_attempts": 0, "last_updated_at": now}}

	_, err = models.UsersModel.UpdateOne(ctxLock, filter, lock)
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE014")
	}

	return cmm.NewEmptyErrorHandler()
}

// RegisterSuccessfulLoginService guarda la fecha del login y limpia los intentos fallidos y el bloqueo
func RegisterSuccessfulLoginService(idUser primitive.ObjectID, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idUser}
	update := bson.M{"$set": bson.M{
		"last_login_at":         now,
		"failed_login_attempts": 0,
		"locked_until":          nil,
		"last_updated_at":       now,
	}}

	_, err := models.UsersModel.UpdateOne(ctx, filter, update)
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE015")
	}

	return cmm.NewEmptyErrorHandler()
}

// UpdateUserPasswordService guarda el nuevo hash y desbloquea el usuario. Las sesiones se cierran aparte con
// DeleteUserSessionsService
func UpdateUserPasswordService(idUser primitive.ObjectID, passwordHash string, now time.Time) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idUser, "logical_delete": false}
	update := bson.M{"$set": bson.M{
		"password_hash":         passwordHash,
		"password_updated_at":   now,
		"failed_login_attempts": 0,
		"locked_until":          nil,
		"last_updated_at":       now,
	}}

	_, err := models.UsersModel.UpdateOne(ctx, filter, update)
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE016")
	}

	return cmm.NewEmptyErrorHandler()
}

// ---------------------------------------------------------------------------
// Base de datos: sesiones
// ---------------------------------------------------------------------------

// CreateSessionService inserta la sesion
func CreateSessionService(session models.SessionsModelType) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	_, err := models.SessionsModel.InsertOne(ctx, session)
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE017")
	}

	return cmm.NewEmptyErrorHandler()
}

// GetActiveSessionService busca la sesion no vencida con el hash del token. El indice TTL puede tardar en borrar
// las vencidas, por eso se filtra por expires_at. Si no existe retorna la sesion vacia sin error
func GetActiveSessionService(tokenHash string, now time.Time) (models.SessionsModelType, *cmm.ErrorHandler) {
	var result models.SessionsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"token_hash": tokenHash, "expires_at": bson.M{"$gt": now}, "logical_delete": false}

	err := models.SessionsModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE018")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// DeleteSessionService borra la sesion (logout del dispositivo actual)
func DeleteSessionService(idSession primitive.ObjectID) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	_, err := models.SessionsModel.DeleteOne(ctx, bson.M{"_id": idSession})
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE019")
	}

	return cmm.NewEmptyErrorHandler()
}

// DeleteUserSessionsService borra todas las sesiones del usuario (logout en todos los dispositivos)
func DeleteUserSessionsService(idUser primitive.ObjectID) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	_, err := models.SessionsModel.DeleteMany(ctx, bson.M{"id_user": idUser})
	if err != nil {
		return cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SAUTE020")
	}

	return cmm.NewEmptyErrorHandler()
}
