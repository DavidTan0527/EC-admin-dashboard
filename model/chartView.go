package model

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChartViewHandler struct {
    *HandlerConns
}

type ChartView struct {
    Id       primitive.ObjectID   `bson:"_id"      json:"id"`
    Name     string               `bson:"name"     json:"name"`
    ChartIds []primitive.ObjectID `bson:"chart_id" json:"chartId"`
}

func (handler *ChartViewHandler) GetChartViewList(c echo.Context) error {
    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART_VIEW)

    cur, err := coll.Find(ctx, bson.M{})
    if err != nil {
        return handleMongoErr(c, err)
    }

    result := make([]ChartView, 0)
    if err := cur.All(ctx, &result); err != nil {
        return handleMongoErr(c, err)
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: result,
    })
}

func (handler *ChartViewHandler) LoadChartView(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART_VIEW)

    c.Logger().Infof("Getting chart with id %s", id)

    var chartView ChartView
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&chartView); err != nil {
        return handleMongoErr(c, err)
    }

    collChart := handler.HandlerConns.Db.Collection(COLL_NAME_CHART)

    result := make([]Chart, 0, len(chartView.ChartIds))
    for _, id := range chartView.ChartIds {
        var chart Chart
        if err := collChart.FindOne(ctx, bson.M{ "_id": id }).Decode(&chart); err != nil {
            return handleMongoErr(c, err)
        }

        c.Logger().Debug("Looking at chart: ", chart)

        isAllowed, err := handler.checkChartPerm(chart, userId)
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

func (handler *ChartViewHandler) CreateChartView(c echo.Context) error {
    body := new(ChartView)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    body.Id = primitive.NewObjectID()

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART_VIEW)

    if res, err := coll.InsertOne(ctx, body); err != nil {
        c.Logger().Info(res)
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Cannot save chart view into DB" })
    }

    message := fmt.Sprintf("ChartView '%s' created", body.Id)
    c.Logger().Info(message)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: message,
    })
}

func (handler *ChartViewHandler) EditChartView(c echo.Context) error {
    body := new(ChartView)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: err.Error() })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART_VIEW)

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

func (handler *ChartViewHandler) DeleteChartView(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection(COLL_NAME_CHART_VIEW)

    if _, err := coll.DeleteOne(ctx, bson.M{"_id": id }); err != nil {
        return handleMongoErr(c, err)
    }

    c.Logger().Infof("ChartView %s deleted", id)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Successfully deleted chart view",
    })
}

func (handler *ChartViewHandler) checkChartPerm(chart Chart, userId string) (bool, error) {
    if chart.PermKey == "" {
        return true, nil
    }
    return checkPerm(handler.HandlerConns, userId, chart.PermKey)
}

