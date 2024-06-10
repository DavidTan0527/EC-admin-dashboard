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

    view, err := handler.canView(userId, body.Key)
    if err != nil {
        c.JSON(http.StatusInternalServerError, echo.Map{ "success": false, "message": "Error getting view permission" })
        return err
    }

    edit, err := handler.canEdit(userId, body.Key)
    if err != nil {
        c.JSON(http.StatusInternalServerError, echo.Map{ "success": false, "message": "Error getting edit permission" })
        return err
    }

    c.JSON(http.StatusOK, echo.Map{ "success": true, "data": Permission{ view, edit } })

    return nil
}

func (handler *PermissionHandler) SetPerm(c echo.Context) error {
    c.String(http.StatusOK, "set perm")
    return nil
}

func (handler *PermissionHandler) canView(userId string, key string) (bool, error) {
    return handler.isMember("user:" + userId, PERM_SET_KEY_PREFIX + key + "_View")
}

func (handler *PermissionHandler) canEdit(userId string, key string) (bool, error) {
    return handler.isMember("user:" + userId, PERM_SET_KEY_PREFIX + key + "_Edit")
}

func (handler *PermissionHandler) isMember(member string, key string) (bool, error) {
    ctx := context.Background()
    cmd := handler.HandlerConns.Redis.SIsMember(ctx, key, member)
    return cmd.Result()
}

