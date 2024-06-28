package model

import (
	"context"
	"net/http"
	"sort"
	"strings"

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
const USER_PREFIX = "user:"

func (handler *PermissionHandler) CheckPerm(c echo.Context) error {
    claims := GetJwtClaims(c)
    userId := claims.UserId

    key := c.Param("key")

    perm, err := CheckPerm(handler.HandlerConns, userId, key)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error checking permission key" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Data: perm })
}

type SetPermBody struct {
    UserId string `json:"user_id" validate:"omitempty,hexadecimal"`
    Key    string `json:"key" validate:"required"`
}

func (handler *PermissionHandler) SetPerm(c echo.Context) error {
    body := new(SetPermBody)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    if body.UserId == "" {
        claims := GetJwtClaims(c)
        body.UserId = claims.UserId
    }

    ctx := context.Background()
    cmd := handler.HandlerConns.Redis.SAdd(ctx, PERM_SET_KEY_PREFIX + body.Key, USER_PREFIX + body.UserId)

    if _, err := cmd.Result(); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error adding permission key" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "Added permission key " + body.Key })
}

func (handler *PermissionHandler) RemovePerm(c echo.Context) error {
    body := new(SetPermBody)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    if body.UserId == "" {
        claims := GetJwtClaims(c)
        body.UserId = claims.UserId
    }

    ctx := context.Background()
    cmd := handler.HandlerConns.Redis.SRem(ctx, PERM_SET_KEY_PREFIX + body.Key, USER_PREFIX + body.UserId)

    if _, err := cmd.Result(); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error removing permission key" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "Removed permission key " + body.Key })
}

func (handler *PermissionHandler) GetAllPermKey(c echo.Context) error {
    ctx := context.Background()

    keys := make([]string, 0)
    var cursor uint64 = 0

    for {
        cmd := handler.HandlerConns.Redis.Scan(ctx, cursor, PERM_SET_KEY_PREFIX + "*", 0)
        result, cursor, err := cmd.Result()
        if err != nil {
            c.Logger().Error(err)
            return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error scanning permission keys" })
        }

        for _, key := range result {
            keys = append(keys, strings.TrimPrefix(key, PERM_SET_KEY_PREFIX))
        }

        if cursor == 0 {
            break
        }
    }

    sort.Strings(keys)

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Data: keys,
        Message: "Successfully get all page permission keys",
    })
}

func (handler *PermissionHandler) GetPermUserList(c echo.Context) error {
    ctx := context.Background()

    key := c.Param("key")
    var result []string

    cmd := handler.HandlerConns.Redis.SMembers(ctx, PERM_SET_KEY_PREFIX + key)
    users, err := cmd.Result()
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error getting users" })
    }

    for _, user := range users {
        result = append(result, strings.TrimPrefix(user, USER_PREFIX))
    }

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Success",
        Data: result,
    })
}

func CheckPerm(handlerConns *HandlerConns, userId string, key string) (bool, error) {
    ctx := context.Background()
    cmd := handlerConns.Redis.SIsMember(ctx, PERM_SET_KEY_PREFIX + key, USER_PREFIX + userId)
    return cmd.Result()
}

