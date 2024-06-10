package main

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func initDb() *mongo.Database {
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    uri := os.Getenv("MONGODB_URI")
    opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

    // Create a new client and connect to the server
    client, err := mongo.Connect(context.TODO(), opts)
    if err != nil {
        panic(err)
    }

    db := client.Database("ec-century")
    return db
}

func initRedis() *redis.Client {
    uri := os.Getenv("REDIS_URI")
    opt, err := redis.ParseURL(uri)
    if err != nil {
        panic(err)
    }

    return redis.NewClient(opt)
}

