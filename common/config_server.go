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

func ConnectDB() *mongo.Database {
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

	fmt.Println("Connection to DB", os.Getenv("JM_CTRD_MONGODB_DATABASE_NAME"))


	database := client.Database("JM_CTRD_MONGODB_DATABASE_NAME")
	
	Database = database


	return database
}

func GetCollection(collectionName string) (*mongo.Collection, error) {
	if collectionName == "" {
		return nil, fmt.Errorf("collection name can not be empty.")
	}
	return Database.Collection(collectionName), nil
}