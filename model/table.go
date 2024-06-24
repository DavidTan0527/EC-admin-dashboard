package model

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TableHandler struct {
    *HandlerConns
}

type Table struct {
    Name   string                   `bson:"name" json:"name" validate:"required"`
    Fields []string                 `bson:"fields" json:"fields" validate:"required"`
    Rows   []map[string]interface{} `bson:"rows" json:"rows" validate:"required"`
}

func (handler *TableHandler) GetTableList(c echo.Context) error {
    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    opts := options.Find().SetProjection(bson.M{ "name": 1 })
    cur, err := coll.Find(ctx, bson.M{}, opts)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    data := make([]struct{ Name string `bson:"name"` }, 0)
    err = cur.All(ctx, &data)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error reading result" })
    }

    tables := make([]string, 0)
    for _, obj := range data {
        tables = append(tables, obj.Name)
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: tables,
    })
}

func (handler *TableHandler) GetTable(c echo.Context) error {
    name := c.Param("table")

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    table := new(Table)
    if err := coll.FindOne(ctx, bson.M{"name": name}).Decode(table); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table not found" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    c.Logger().Infof("Getting table %s", table.Name)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: table,
    })
}

func (handler *TableHandler) SetTable(c echo.Context) error {
    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "name": body.Name }
    opt := options.FindOneAndReplace().SetUpsert(true)

    result := coll.FindOneAndReplace(ctx, filter, body, opt)
    if result.Err() != nil {
        c.Logger().Error(result.Err().Error())
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    c.Logger().Infof("Updating/Creating table %s", body.Name)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Table updated",
    })
}


