package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"

	"github.com/DavidTan0527/EC-admin-dashboard/server/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserHandler struct {
    *HandlerConns
}

type User struct {
    Id       primitive.ObjectID `bson:"_id" json:"id"`
    Username string             `bson:"username" json:"username"`
    Password string             `bson:"password" json:"password"`
    Salt     string             `bson:"salt" json:"salt"`
    IsSuper  bool               `bson:"is_super" json:"is_super"`
}

func (handler *UserHandler) LoginUser(c echo.Context) error {
    data, err := GetRequestBody(c)
    if err != nil {
        return err
    }

    username, ok := data["username"].(string)
    if !ok {
        c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: "'username' missing" })
        return echo.ErrBadRequest
    }

    password, ok := data["password"].(string)
    if !ok {
        c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: "'password' missing" })
        return echo.ErrBadRequest
    }

    userColl := handler.HandlerConns.Db.Collection("User")

    user := new(User)
    if err = userColl.FindOne(context.Background(), bson.M{"username": username}).Decode(user); err != nil {
        if err == mongo.ErrNoDocuments {
            c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username or password incorrect" })
            return echo.ErrUnauthorized
        }
        return err
    }

    salt, err := hex.DecodeString(user.Salt)
    if err != nil {
        return err
    }
    saltedPwd := append([]byte(password), salt...)
    pwdHash, err := sha256Hash(saltedPwd)
    pwd := hex.EncodeToString(pwdHash)

    if pwd != user.Password {
        c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username or password incorrect" })
        return nil
    }

    claims := auth.NewJwtClaims()
    claims.UserId = user.Id.String()
    claims.IsSuper = user.IsSuper
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims) 

    t, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
    fmt.Println(os.Getenv("JWT_SECRET"))
    if err != nil {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error generating JWT token" })
        return err
    }

    c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Login successful",
        Data: echo.Map{ "token": t },
    })

    return nil
}



func (handler *UserHandler) CreateUser(c echo.Context) error {
    data, err := GetRequestBody(c)
    if err != nil {
        return err
    }

    username, ok := data["username"].(string)
    if !ok {
        c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: "'username' missing" })
        return echo.ErrBadRequest
    }

    password, ok := data["password"].(string)
    if !ok {
        c.JSON(http.StatusBadRequest, HttpResponseBody{ Success: false, Message: "'password' missing" })
        return echo.ErrBadRequest
    }

    isSuper, ok := data["is_super"].(bool)
    if !ok {
        isSuper = false
    }

    claims := GetJwtClaims(c)
    if isSuper && !claims.IsSuper {
        c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "Only super users can create super users" })
        return echo.ErrUnauthorized
    }

    user := new(User)
    user.Id = primitive.NewObjectID()
    user.Username = username
    user.IsSuper = isSuper

    salt := make([]byte, 32)
    if _, err = rand.Read(salt); err != nil {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error generating salt" })
        return err
    }
    user.Salt = hex.EncodeToString(salt)

    saltedPwd := append([]byte(password), salt...)
    pwdHash, err := sha256Hash(saltedPwd)
    if err != nil {
        c.JSON(http.StatusInternalServerError, echo.Map{
            "success": false,
            "message": "Error hashing password",
        })
        return err
    }
    user.Password = hex.EncodeToString(pwdHash)

    coll := handler.HandlerConns.Db.Collection("User")
    if err := coll.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(new(User)); err == nil {
        c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username exists" })
        return nil
    } else if err != mongo.ErrNoDocuments {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
        return err
    }

    fmt.Println("Creating user " + user.Username + " with password hash " + user.Password)

    if res, err := coll.InsertOne(context.Background(), user); err != nil {
        fmt.Println(res)
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Cannot save user into DB" })
        return err
    }
    c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: true, Message: "User " + user.Username + " created"})

    return nil
}



func (handler *UserHandler) GetUser(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        return err
    }

    fmt.Println("Getting user with id " + id.String())

    user := new(User)

    coll := handler.HandlerConns.Db.Collection("User")
    result := coll.FindOne(context.Background(), bson.M{"_id": id})
    err = result.Decode(user)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            c.JSON(http.StatusOK, HttpResponseBody{
                Success: false,
                Message: "User not found",
            })
            return nil
        }
        return err
    }

    c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "User not found",
        Data: user,
    })

    return nil
}

func sha256Hash(input []byte) ([]byte, error) {
    hasher := sha256.New()
    if _, err := hasher.Write(input); err != nil {
        return nil, err
    }

    return hasher.Sum(nil), nil
}

