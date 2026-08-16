package common

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Database client
var Database *mongo.Database

type ModelRegistry struct {
	Name string
	Ptr  **mongo.Collection
}


func ConnectDB(modelsToInit []ModelRegistry ) *mongo.Database {
	err := godotenv.Load()


	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Obtener URI de .env (asegúrate de agregarla)
	uri := os.Getenv("JM_CTRD_MONGODB_SERVER_URL")

	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))

	if err != nil {
		log.Fatal(err)
	}

	// Verificar conexión
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Connection failed to MongoDB:", err)
	}

	fmt.Println("Connection to MongoDB succesfull")

	dbName := os.Getenv("JM_CTRD_MONGODB_DATABASE_NAME")

	if dbName == "" {
		dbName = "c_trd_default_db" // Valor por defecto por si olvidas ponerlo en el .env
	}


	fmt.Println("Connection to DB", dbName)


	database := client.Database(dbName)
	
	Database = database

	for _, model := range modelsToInit {
		*model.Ptr = database.Collection(model.Name)
	}

	fmt.Println("All models initialized abstractly.")

	return database
}

func GetCollection(collectionName string) (*mongo.Collection) {
	if collectionName == "" {
		panic(fmt.Errorf("collection name can not be empty."))
	}
	return Database.Collection(collectionName)
}