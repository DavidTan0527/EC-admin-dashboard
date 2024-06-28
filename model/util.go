package model

import (
	"crypto/sha256"
	"net/http"

	"github.com/DavidTan0527/EC-admin-dashboard/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
)

func Ping(c echo.Context) error {
    c.JSON(http.StatusOK, HttpResponseBody{ Success: true })
    return nil
}

func GetRequestBody(c echo.Context, body interface{}) error {
    if err := c.Bind(body); err != nil {
      return err
    }
    if err := c.Validate(body); err != nil {
      return err
    }
    return nil
}

func GetJwtClaims(c echo.Context) *auth.JwtClaims {
    token := c.Get("user").(*jwt.Token)
    claims := token.Claims.(*auth.JwtClaims)
    return claims
}

func handleMongoErr(c echo.Context, err error) error {
    if err == mongo.ErrNoDocuments {
        return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "ID does not exist" })
    }
    c.Logger().Error(err)
    return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
}

func sha256Hash(input []byte) ([]byte, error) {
    hasher := sha256.New()
    if _, err := hasher.Write(input); err != nil {
        return nil, err
    }

    return hasher.Sum(nil), nil
}

