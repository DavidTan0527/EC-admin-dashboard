package model

import (
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
    COLL_NAME_USER = "User"
    COLL_NAME_TABLE = "Table"
    COLL_NAME_CHART = "Chart"
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

