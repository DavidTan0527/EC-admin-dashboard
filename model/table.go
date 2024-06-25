package model

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TableHandler struct {
    *HandlerConns
}

type Table struct {
    Id     primitive.ObjectID       `bson:"_id,omitempty" json:"id,omitempty"`
    Name   string                   `bson:"name" json:"name"`
    Fields []map[string]interface{} `bson:"fields" json:"fields"`
    Rows   []map[string]interface{} `bson:"rows" json:"rows"`
}

func (handler *TableHandler) GetTableList(c echo.Context) error {
    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    opts := options.Find().SetProjection(bson.M{ "_id": 1, "name": 1 })
    cur, err := coll.Find(ctx, bson.M{}, opts)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    data := make([]struct{
        Id   primitive.ObjectID `bson:"_id" json:"id"`
        Name string `bson:"name" json:"name"`
    }, 0)
    err = cur.All(ctx, &data)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error reading result" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: data,
    })
}

func (handler *TableHandler) GetTable(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    c.Logger().Infof("Getting table with id %s", id)

    table := new(Table)
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(table); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table not found" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: table,
    })
}

func (handler *TableHandler) CreateTable(c echo.Context) error {
    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    body.Id = primitive.NewObjectID()

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if err := coll.FindOne(ctx, bson.M{"name": body.Name}).Decode(new(Table)); err == nil {
        return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table already exists" })
    } else if err != mongo.ErrNoDocuments {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    if res, err := coll.InsertOne(ctx, body); err != nil {
        c.Logger().Info(res)
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Cannot save table into DB" })
    }

    message := fmt.Sprintf("Table %s created", body.Name)
    c.Logger().Info(message)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: message,
    })
}

func (handler *TableHandler) EditTable(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "_id": id }
    update := bson.M{
        "$set": bson.M{
            "fields": body.Fields,
            "rows": body.Rows,
        },
    }

    c.Logger().Infof("Updating table %s", id)

    if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Table does not exist" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Changes saved",
    })

}

func (handler *TableHandler) RenameTable(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "_id": id }
    update := bson.M{ "$set": bson.M{ "name": body.Name } }

    c.Logger().Infof("Renaming table %s to %s", id, body.Name)

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
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if _, err := coll.DeleteOne(ctx, bson.M{"_id": id }); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error deleting user" })
    }

    c.Logger().Infof("Table %s deleted", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Successfully deleted table",
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
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "_id": id }
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

    c.Logger().Infof("Getting schema of table %s", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: res,
    })
}

