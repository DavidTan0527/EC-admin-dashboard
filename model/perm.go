package model

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

type PermissionHandler struct {
    *HandlerConns
}

type Permission struct {
    CanView bool `json:"can_view"`
    CanEdit bool `json:"can_edit"`
}

const PERM_SET_KEY_PREFIX = "ec:permission:"

type CheckPermBody struct {
    Key string `json:"key" validate:"required"`
}

func (handler *PermissionHandler) CheckPerm(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    body := new(CheckPermBody)
    if err := GetRequestBody(c, body); err != nil {
        return err
    }

    perm, err := handler.isMember("user:" + userId, PERM_SET_KEY_PREFIX + body.Key)
    if err != nil {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
        return err
    }

    c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Data: perm })

    return nil
}

func (handler *PermissionHandler) SetPerm(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    body := new(CheckPermBody)
    if err := GetRequestBody(c, body); err != nil {
        return err
    }

    ctx := context.Background()
    cmd := handler.HandlerConns.Redis.SAdd(ctx, PERM_SET_KEY_PREFIX + body.Key, "user:" + userId)

    if _, err := cmd.Result(); err != nil {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error adding permission key" })
        return err
    }

    c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "Added permission key " + body.Key })

    return nil
}

func (handler *PermissionHandler) isMember(member string, key string) (bool, error) {
    ctx := context.Background()
    cmd := handler.HandlerConns.Redis.SIsMember(ctx, key, member)
    return cmd.Result()
}

