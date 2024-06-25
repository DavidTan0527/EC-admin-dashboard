package model

import (
	"context"
	"fmt"
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
    Fields []string                 `bson:"fields" json:"fields"`
    Rows   []map[string]interface{} `bson:"rows" json:"rows"`
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
    opt := options.Replace().SetUpsert(true)

    result, err := coll.ReplaceOne(ctx, filter, body, opt)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    var message string

    if result.UpsertedCount == 0 {
        message = fmt.Sprintf("Table %s created", body.Name)
    } else {
        message = fmt.Sprintf("Table %s updated", body.Name)
    }

    c.Logger().Info(message)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: message,
    })
}

func (handler *TableHandler) RenameTable(c echo.Context) error {
    name := c.Param("table")

    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "name": name }
    update := bson.M{ "$set": bson.M{ "name": body.Name } }

    c.Logger().Infof("Renaming table %s to %s", name, body.Name)

    if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table does not exist" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Edited",
    })
}

func (handler *TableHandler) DeleteTable(c echo.Context) error {
    name := c.Param("table")

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if _, err := coll.DeleteOne(ctx, bson.M{"name": name }); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error deleting user" })
    }

    c.Logger().Infof("Table %s deleted", name)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Successfully deleted table " + name,
    })
}

func (handler *TableHandler) GetAllTableSchema(c echo.Context) error {
    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    opt := options.Find().SetProjection(bson.M{ "fields": 1 })

    res := make([]struct {
        Fields string `bson:"fields" json:"fields"`
    }, 0)

    cursor, err := coll.Find(ctx, bson.D{}, opt)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table does not exist" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    if err = cursor.All(ctx, res); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    c.Logger().Info("Getting schema of all tables")

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: res,
    })
}

func (handler *TableHandler) GetTableSchema(c echo.Context) error {
    name := c.Param("table")

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "name": name }
    opt := options.FindOne().SetProjection(bson.M{ "fields": 1 })

    var res struct {
        Fields string `bson:"fields" json:"fields"`
    }
    if err := coll.FindOne(ctx, filter, opt).Decode(&res); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table does not exist" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    c.Logger().Infof("Getting schema of table %s", name)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: res,
    })
}

