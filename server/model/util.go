package model

import (
	"encoding/json"

	"github.com/DavidTan0527/EC-admin-dashboard/server/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func GetRequestBody(c echo.Context) (echo.Map, error) {
    body := c.Request().Body
    data := make(echo.Map)
    err := json.NewDecoder(body).Decode(&data)
    if err != nil {
        return nil, err
    }
    return data, nil
}

func GetJwtClaims(c echo.Context) *auth.JwtClaims {
    token := c.Get("user").(*jwt.Token)
    claims := token.Claims.(*auth.JwtClaims)
    return claims
}

