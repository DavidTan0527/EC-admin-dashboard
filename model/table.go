package model

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TableHandler struct {
    *HandlerConns
}

type ObjArray = []map[string]interface{}

type Table struct {
    Id      primitive.ObjectID                       `bson:"_id,omitempty" json:"id"`
    Name    string                                   `bson:"name" json:"name"`
    PermKey string                                   `bson:"perm_key" json:"permKey"`
    SortKey int                                      `bson:"sort_key" json:"sortKey"`
    Fields  ObjArray                                 `bson:"fields" json:"fields"`
    Data    map[string]map[string]primitive.ObjectID `bson:"data" json:"data"`
}

type TableFull struct {
    Id      primitive.ObjectID             `json:"id"`
    Name    string                         `json:"name"`
    PermKey string                         `json:"permKey"`
    Fields  ObjArray                       `json:"fields"`
    Data    map[string]map[string]ObjArray `json:"data"`
}

type TableData struct {
    Id   primitive.ObjectID `bson:"_id"`
    Rows ObjArray           `bson:"rows"`
}

type HttpTable struct {
    Id      primitive.ObjectID `json:"id,omitempty"`
    Name    string             `json:"name"`
    PermKey string             `json:"permKey"`
    Fields  ObjArray           `json:"fields"`
    Rows    ObjArray           `json:"rows"`
}

var TABLE_BASIC_PROJECTION = bson.M{ "_id": 1, "name": 1, "perm_key": 1 }
var TABLE_PERM_PROJECTION = bson.M{ "perm_key": 1 }
var TABLE_METADATA_PROJECTION = bson.M{ "_id": 1, "name": 1, "perm_key": 1, "fields": 1 }

var SORT_FIELDS = bson.M{ "sort_key": 1 }

func (handler *TableHandler) GetTableList(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    opts := options.Find().SetProjection(TABLE_BASIC_PROJECTION).SetSort(SORT_FIELDS)
    cur, err := coll.Find(ctx, bson.M{}, opts)
    if err != nil {
        return handleMongoErr(c, err)
    }

    result := make([]HttpTable, 0)

    for cur.Next(ctx) {
        var table Table
        if err := cur.Decode(&table); err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
        }

        isAllowed, err := handler.checkTablePerm(table, userId)
        if err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
        }

        if isAllowed {
            result = append(result, HttpTable{
                Id: table.Id,
                Name: table.Name,
                PermKey: table.PermKey,
            })
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
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    year := c.Param("year")
    month := c.Param("month")

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    c.Logger().Infof("Getting table with id %s", id)

    var table Table
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&table); err != nil {
        return handleMongoErr(c, err)
    }

    isAllowed, err := handler.checkTablePerm(table, userId)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    if !isAllowed {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission to view this table" })
    }


    var data HttpTable

    data.Id = table.Id
    data.PermKey = table.PermKey
    data.Name = table.Name
    data.Fields = table.Fields
    data.Rows, _, err = handler.fetchTableRows(table, year, month)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: data,
    })
}

func (handler *TableHandler) GetTableFull(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    c.Logger().Infof("Getting table with id %s", id)

    var table Table
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&table); err != nil {
        return handleMongoErr(c, err)
    }

    isAllowed, err := handler.checkTablePerm(table, userId)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    if !isAllowed {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission to view this table" })
    }


    var data TableFull

    data.Id = table.Id
    data.PermKey = table.PermKey
    data.Name = table.Name
    data.Fields = table.Fields
    data.Data = make(map[string]map[string]ObjArray)

    var wg sync.WaitGroup
    mutex := &sync.RWMutex{}
    errs := make(chan error)
    for year, yearIds := range table.Data {
        if data.Data[year] == nil {
            data.Data[year] = make(map[string]ObjArray)
        }
        for month := range yearIds {
            wg.Add(1)
            go func(year, month string) {
                defer wg.Done()

                rows, _, err := handler.fetchTableRows(table, year, month)

                mutex.Lock()
                data.Data[year][month] = rows
                mutex.Unlock()

                if err != nil {
                    errs <- err
                }
            }(year, month)
        }
    }
    wg.Wait()
    if len(errs) != 0 {
        var message string
        for len(errs) > 0 {
            message += (<-errs).Error() + "\n"
        }
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: message })
    }



    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: data,
    })
}

func (handler *TableHandler) CreateTable(c echo.Context) error {
    body := new(Table)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    body.Id = primitive.NewObjectID()
    if body.Fields == nil {
        body.Fields = make([]map[string]interface{}, 0)
    }
    if body.Data == nil {
        body.Data = make(map[string]map[string]primitive.ObjectID)
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    if res, err := coll.InsertOne(ctx, body); err != nil {
        c.Logger().Info(res)
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
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
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    year := c.Param("year")
    month := c.Param("month")

    claims := GetJwtClaims(c)
    userId := claims.UserId

    body := new(HttpTable)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    var table Table 
    if err := coll.FindOne(ctx, bson.M{ "_id": id }).Decode(&table); err != nil {
        return handleMongoErr(c, err)
    }

    if perm, err := handler.checkTablePerm(table, userId); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    } else if !perm {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission" })
    }

    if err := handler.updateTableData(table, year, month, body); err != nil {
        return handleMongoErr(c, err)
    }

    c.Logger().Infof("Updating table %s", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Changes saved",
    })

}

func (handler *TableHandler) EditTableMetadata(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
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

    claims := GetJwtClaims(c)
    userId := claims.UserId
    if perm, err := handler.fetchCheckTablePerm(id, userId); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    } else if !perm {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission" })
    }

    if _, err := coll.UpdateOne(ctx, filter, update); err != nil {
        return handleMongoErr(c, err)
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Edited",
    })
}

type EditTableSortBody struct {
    SortKey int `json:"sortKey"`
}

func (handler *TableHandler) EditTableSort(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    body := new(EditTableSortBody)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "_id": id }
    update := bson.M{
        "$set": bson.M{
            "sort_key": body.SortKey,
        },
    }

    c.Logger().Infof("Editing table %s sort to %d", id, body.SortKey)

    claims := GetJwtClaims(c)
    userId := claims.UserId
    if perm, err := handler.fetchCheckTablePerm(id, userId); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: err.Error() })
    } else if !perm {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission" })
    }

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

    if perm, err := handler.fetchCheckTablePerm(id, userId); err != nil {
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

    opt := options.Find().SetProjection(TABLE_METADATA_PROJECTION).SetSort(SORT_FIELDS)
    cur, err := coll.Find(ctx, bson.D{}, opt)
    if err != nil {
        return handleMongoErr(c, err)
    }

    result := make([]HttpTable, 0)

    for cur.Next(ctx) {
        var table Table
        if err := cur.Decode(&table); err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
        }

        isAllowed, err := handler.checkTablePerm(table, userId)
        if err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
        }

        if isAllowed {
            result = append(result, HttpTable{
                Id: table.Id,
                Name: table.Name,
                PermKey: table.PermKey,
                Fields: table.Fields,
            })
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

    if perm, err := handler.fetchCheckTablePerm(id, userId); err != nil {
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

    response := HttpTable{
        Id: table.Id,
        Name: table.Name,
        PermKey: table.PermKey,
        Fields: table.Fields,
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: response,
    })
}

func (handler *TableHandler) fetchCheckTablePerm(id primitive.ObjectID, userId string) (bool, error) {
    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)

    filter := bson.M{ "_id": id }
    opt := options.FindOne().SetProjection(TABLE_PERM_PROJECTION)

    var table Table 
    if err := coll.FindOne(ctx, filter, opt).Decode(&table); err != nil {
        return false, err
    }
    
    return handler.checkTablePerm(table, userId)
}

func (handler *TableHandler) checkTablePerm(table Table, userId string) (bool, error) {
    if table.PermKey == "" {
        return true, nil
    }
    return checkPerm(handler.HandlerConns, userId, table.PermKey)
}

func (handler *TableHandler) fetchTableRows(table Table, year string, month string) (ObjArray, bool, error) {
    if table.Data == nil {
        return make(ObjArray, 0), false, nil
    }

    yearIds, ok := table.Data[year]
    if !ok {
        return make(ObjArray, 0), false, nil
    }

    dataId, ok := yearIds[month]
    if !ok {
        return make(ObjArray, 0), false, nil
    }

    var data TableData
    ctx := context.Background()

    dataColl := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE_DATA)
    if err := dataColl.FindOne(ctx, bson.M{ "_id": dataId }).Decode(&data); err == mongo.ErrNoDocuments {
        // Allow document not found
        return make(ObjArray, 0), false, nil
    } else if err != nil {
        return make(ObjArray, 0), false, err
    }

    return data.Rows, true, nil
}

func (handler *TableHandler) updateTableData(table Table, year string, month string, body *HttpTable) error {
    _, found, err := handler.fetchTableRows(table, year, month)
    if err != nil {
        return err
    }

    if !found {
        if table.Data == nil {
            table.Data = make(map[string]map[string]primitive.ObjectID)
        }

        if _, ok := table.Data[year]; !ok {
            table.Data[year] = make(map[string]primitive.ObjectID)
        }

        table.Data[year][month] = primitive.NewObjectID()
    }

    table.Fields = body.Fields

    ctx := context.Background()

    coll := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE)
    tableUpdate := bson.M {
        "$set": bson.M{
            "fields": table.Fields,
            "data": table.Data,
        },
    }
    if _, err := coll.UpdateByID(ctx, table.Id, tableUpdate); err != nil {
        return err
    }

    dataColl := handler.HandlerConns.Db.Collection(COLL_NAME_TABLE_DATA)
    opts := options.Update().SetUpsert(true)
    update := bson.M{
        "$set": bson.M{
            "rows": body.Rows,
        },
    }
    if _, err := dataColl.UpdateByID(ctx, table.Data[year][month], update, opts); err != nil {
        return err
    }

    return nil
}

