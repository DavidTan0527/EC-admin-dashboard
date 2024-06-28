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

type ChartHandler struct {
    *HandlerConns
}

type Chart struct {
    Id      primitive.ObjectID     `bson:"_id"      json:"id"`
    Type    string                 `bson:"type"     json:"type"    validate:"required"`
    Title   string                 `bson:"title"    json:"title"   validate:"required"`
    PermKey string                 `bson:"perm_key" json:"permKey" validate:"required"`
    TableId primitive.ObjectID     `bson:"table_id" json:"tableId" validate:"required"`
    Options map[string]interface{} `bson:"options"  json:"options" validate:"required"`
}

var CHART_PERM_PROJECTION = bson.M{ "perm_key": 1 }

func (handler *ChartHandler) GetChart(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART)

    c.Logger().Infof("Getting chart with id %s", id)

    var chart Chart
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&chart); err != nil {
        return handleMongoErr(c, err)
    }

    isAllowed, err := checkChartPerm(handler.HandlerConns, chart, userId)
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
        Data: chart,
    })
}

func (handler *ChartHandler) GetAllChart(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART)

    cur, err := coll.Find(ctx, bson.M{})
    if err != nil {
        return handleMongoErr(c, err)
    }

    result := make([]Chart, 0)

    for cur.Next(ctx) {
        var chart Chart
        if err := cur.Decode(&chart); err != nil {
            return handleMongoErr(c, err)
        }

        c.Logger().Debug("Looking at chart: ", chart)

        isAllowed, err := checkChartPerm(handler.HandlerConns, chart, userId)
        if err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
        }

        if isAllowed {
            result = append(result, chart)
        }
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: result,
    })
}

func (handler *ChartHandler) CreateChart(c echo.Context) error {
    body := new(Chart)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    body.Id = primitive.NewObjectID()

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART)

    if res, err := coll.InsertOne(ctx, body); err != nil {
        c.Logger().Info(res)
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Cannot save chart into DB" })
    }

    message := fmt.Sprintf("Chart '%s' created", body.Title)
    c.Logger().Info(message)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: message,
    })
}

func (handler *ChartHandler) EditChart(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    body := new(Chart)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART)

    isAllowed, err := fetchCheckChartPerm(handler.HandlerConns, body.Id, userId)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    }

    if !isAllowed {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission to view this table" })
    }

    filter := bson.M{ "_id": body.Id }

    res, err := coll.ReplaceOne(ctx, filter, body)
    if err != nil {
        return handleMongoErr(c, err)
    }

    c.Logger().Info("Chart edited:", res)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Edited",
    })
}

func (handler *ChartHandler) DeleteChart(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART)

    isAllowed, err := fetchCheckChartPerm(handler.HandlerConns, id, userId)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    }

    if !isAllowed {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "No permission to view this table" })
    }

    if _, err := coll.DeleteOne(ctx, bson.M{"_id": id }); err != nil {
        return handleMongoErr(c, err)
    }

    c.Logger().Infof("Chart %s deleted", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Successfully deleted chart",
    })
}

func fetchCheckChartPerm(handlerConns *HandlerConns, id primitive.ObjectID, userId string) (bool, error) {
    ctx := context.Background()
    coll := handlerConns.Db.Collection(COLL_NAME_CHART)

    filter := bson.M{ "_id": id }
    opt := options.FindOne().SetProjection(CHART_PERM_PROJECTION)

    var chart Chart 
    if err := coll.FindOne(ctx, filter, opt).Decode(&chart); err != nil {
        return false, err
    }
    
    return checkChartPerm(handlerConns, chart, userId)
}

func checkChartPerm(handlerConns *HandlerConns, chart Chart, userId string) (bool, error) {
    if chart.PermKey == "" {
        return true, nil
    }
    return CheckPerm(handlerConns, userId, chart.PermKey)
}

