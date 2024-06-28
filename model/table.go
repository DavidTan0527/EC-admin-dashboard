package model

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TableHandler struct {
    *HandlerConns
}

type Table struct {
    Id      primitive.ObjectID       `bson:"_id,omitempty" json:"id,omitempty"`
    Name    string                   `bson:"name" json:"name"`
    PermKey string                   `bson:"perm_key" json:"permKey"`
    Fields  []map[string]interface{} `bson:"fields" json:"fields"`
    Rows    []map[string]interface{} `bson:"rows" json:"rows"`
}

var TABLE_BASIC_PROJECTION = bson.M{ "_id": 1, "name": 1, "perm_key": 1 }
var TABLE_PERM_PROJECTION = bson.M{ "perm_key": 1 }
var TABLE_METADATA_PROJECTION = bson.M{ "_id": 1, "name": 1, "perm_key": 1, "fields": 1 }

func (handler *TableHandler) GetTableList(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    opts := options.Find().SetProjection(TABLE_BASIC_PROJECTION)
    cur, err := coll.Find(ctx, bson.M{}, opts)
    if err != nil {
        return handleMongoErr(c, err)
    }

    result := make([]Table, 0)

    for cur.Next(ctx) {
        var table Table
        if err := cur.Decode(&table); err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
        }

        isAllowed, err := checkTablePerm(handler.HandlerConns, table, userId)
        if err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
        }

        if isAllowed {
            result = append(result, table)
        }
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: result,
    })
}

func (handler *TableHandler) GetTable(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    c.Logger().Infof("Getting table with id %s", id)

    var table Table
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&table); err != nil {
        return handleMongoErr(c, err)
    }

    isAllowed, err := checkTablePerm(handler.HandlerConns, table, userId)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    }

    if !isAllowed {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission to view this table" })
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

func (handler *TableHandler) EditTableData(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    claims := GetJwtClaims(c)
    userId := claims.UserId

    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if perm, err := fetchCheckTablePerm(handler.HandlerConns, id, userId); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    } else if !perm {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission" })
    }

    filter := bson.M{ "_id": id }
    update := bson.M{
        "$set": bson.M{
            "fields": body.Fields,
            "rows": body.Rows,
        },
    }

    c.Logger().Infof("Updating table %s", id)

    if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
        return handleMongoErr(c, err)
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Changes saved",
    })

}

func (handler *TableHandler) EditTableMetadata(c echo.Context) error {
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
            "name": body.Name,
            "perm_key": body.PermKey,
        },
    }

    c.Logger().Infof("Editing table %s metadata", id)

    if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
        return handleMongoErr(c, err)
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

    claims := GetJwtClaims(c)
    userId := claims.UserId

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if perm, err := fetchCheckTablePerm(handler.HandlerConns, id, userId); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    } else if !perm {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission" })
    }

    if _, err := coll.DeleteOne(ctx, bson.M{"_id": id }); err != nil {
        return handleMongoErr(c, err)
    }

    c.Logger().Infof("Table %s deleted", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Successfully deleted table",
    })
}

func (handler *TableHandler) GetAllTableSchema(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    opt := options.Find().SetProjection(TABLE_METADATA_PROJECTION)
    cur, err := coll.Find(ctx, bson.D{}, opt)
    if err != nil {
        return handleMongoErr(c, err)
    }

    result := make([]Table, 0)

    for cur.Next(ctx) {
        var table Table
        if err := cur.Decode(&table); err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
        }

        isAllowed, err := checkTablePerm(handler.HandlerConns, table, userId)
        if err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
        }

        if isAllowed {
            result = append(result, table)
        }
    }

    c.Logger().Info("Getting schema of all tables")

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: result,
    })
}

func (handler *TableHandler) GetTableSchema(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if perm, err := fetchCheckTablePerm(handler.HandlerConns, id, userId); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    } else if !perm {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission" })
    }

    filter := bson.M{ "_id": id }
    opt := options.FindOne().SetProjection(TABLE_METADATA_PROJECTION)

    var table Table 
    if err := coll.FindOne(ctx, filter, opt).Decode(&table); err != nil {
        return handleMongoErr(c, err)
    }

    c.Logger().Infof("Getting schema of table %s", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: table,
    })
}

func fetchCheckTablePerm(handlerConns *HandlerConns, id primitive.ObjectID, userId string) (bool, error) {
    ctx := context.Background()
    coll := handlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "_id": id }
    opt := options.FindOne().SetProjection(TABLE_PERM_PROJECTION)

    var table Table 
    if err := coll.FindOne(ctx, filter, opt).Decode(&table); err != nil {
        return false, err
    }
    
    return checkTablePerm(handlerConns, table, userId)
}

func checkTablePerm(handlerConns *HandlerConns, table Table, userId string) (bool, error) {
    if table.PermKey == "" {
        return true, nil
    }
    return CheckPerm(handlerConns, userId, table.PermKey)
}

