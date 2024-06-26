package main

import (
	"github.com/DavidTan0527/EC-admin-dashboard/model"
	"github.com/joho/godotenv"
)

func main() {
    godotenv.Load()

    initRoutes(&model.HandlerConns{
        Db: initDb(),
        Redis: initRedis(),
    })
}

