package model

import (
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type HandlerConns struct {
    Db    *mongo.Database
    Redis *redis.Client
}

type HttpResponseBody struct {
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
}

