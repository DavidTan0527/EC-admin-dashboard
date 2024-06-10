package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

type LoginUserBody struct {
    Username string `json:"username" validate:"required"`
    Password string `json:"password" validate:"required"`
}

func (handler *UserHandler) LoginUser(c echo.Context) error {
    body := new(LoginUserBody)
    if err := GetRequestBody(c, body); err != nil {
      return err
    }

    coll := handler.HandlerConns.Db.Collection("User")

    user := new(User)
    if err := coll.FindOne(context.Background(), bson.M{"username": body.Username}).Decode(user); err != nil {
        if err == mongo.ErrNoDocuments {
            c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username or password incorrect" })
            return nil
        }
        return err
    }

    correct, err := verifyPassword(body.Password, user.Salt, user.Password)
    if err != nil {
        return err
    }
    if !correct {
        c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username or password incorrect" })
        return nil
    }

    claims := auth.NewJwtClaims()
    claims.UserId = user.Id.String()
    claims.IsSuper = user.IsSuper
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims) 

    t, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
    if err != nil {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error generating JWT token" })
        return err
    }

    c.Logger().Info("User " + user.Username + " logged in")

    c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Login successful",
        Data: echo.Map{ "token": t },
    })

    return nil
}

type CreateUserBody struct {
    Username string `json:"username" validate:"required"`
    Password string `json:"password" validate:"required"`
    IsSuper  bool   `json:"is_super" validate:"boolean"`
}

func (handler *UserHandler) CreateUser(c echo.Context) error {
    body := new(CreateUserBody)
    if err := GetRequestBody(c, body); err != nil {
        return err
    }

    claims := GetJwtClaims(c)
    if body.IsSuper && !claims.IsSuper {
        c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "Only super users can create super users" })
        return echo.ErrUnauthorized
    }

    user := new(User)
    user.Id = primitive.NewObjectID()
    user.Username = body.Username
    user.IsSuper = body.IsSuper

    password, salt, err := generateSaltAndPasswordHash(body.Password)
    if err != nil {
        return err
    }
    user.Password = password
    user.Salt = salt

    coll := handler.HandlerConns.Db.Collection("User")
    if err := coll.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(new(User)); err == nil {
        c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username exists" })
        return nil
    } else if err != mongo.ErrNoDocuments {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
        return err
    }

    c.Logger().Info("Creating user " + user.Username + " with password hash " + user.Password)

    if res, err := coll.InsertOne(context.Background(), user); err != nil {
        c.Logger().Info(res)
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

    claims := GetJwtClaims(c)
    if !claims.IsSuper && id.String() != claims.UserId {
        return echo.NewHTTPError(http.StatusUnauthorized, "Non-super users cannot get other users")
    }

    user := new(User)

    coll := handler.HandlerConns.Db.Collection("User")
    result := coll.FindOne(context.Background(), bson.M{"_id": id})
    err = result.Decode(user)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "User not found" })
            return nil
        }
        return err
    }

    c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Data: user })

    return nil
}

func (handler *UserHandler) GetAllUsers(c echo.Context) error {
    claims := GetJwtClaims(c)
    if !claims.IsSuper {
        return echo.NewHTTPError(http.StatusUnauthorized, "Non-super users cannot get all users")
    }

    ctx := context.Background()

    coll := handler.HandlerConns.Db.Collection("User")
    cur, err := coll.Find(ctx, bson.M{})
    if err != nil {
        return err
    }

    data := make([]User, 0)
    err = cur.All(ctx, &data)
    if err != nil {
        c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error reading result" })
        return err
    }

    c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Data: data })

    return nil
}

type UpdateUserPasswordBody struct {
    OldPassword string `json:"old_password" validate:"required"`
    NewPassword string `json:"new_password" validate:"required"`
}

func (handler *UserHandler) UpdateUserPassword(c echo.Context) error {
    body := new(UpdateUserPasswordBody)
    if err := GetRequestBody(c, body); err != nil {
        return err
    }

    claims := GetJwtClaims(c)

    coll := handler.HandlerConns.Db.Collection("User")

    user := new(User)
    if err := coll.FindOne(context.Background(), bson.M{"_id": claims.UserId}).Decode(user); err != nil {
        if err == mongo.ErrNoDocuments {
            c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "User does not exist" })
            return echo.ErrUnauthorized
        }
        return err
    }

    correct, err := verifyPassword(body.OldPassword, user.Salt, user.Password)
    if err != nil {
        return err
    }
    if !correct {
        c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Incorrect old pasword" })
        return nil
    }

    password, salt, err := generateSaltAndPasswordHash(body.NewPassword)
    if err != nil {
        return err
    }
    user.Password = password
    user.Salt = salt

    c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "Password changed successfully" })

    return nil
}

func generateSaltAndPasswordHash(passwordText string) (password string, salt string, err error) {
    saltBytes := make([]byte, 32)
    if _, err = rand.Read(saltBytes); err != nil {
        err = echo.NewHTTPError(http.StatusInternalServerError, "Error generating salt")
        return
    }
    salt = hex.EncodeToString(saltBytes)

    saltedPwd := append([]byte(passwordText), saltBytes...)
    pwdHash, err := sha256Hash(saltedPwd)
    if err != nil {
        err = echo.NewHTTPError(http.StatusInternalServerError, "Error hashing password")
        return
    }
    password = hex.EncodeToString(pwdHash)

    return
}

func verifyPassword(passwordText string, salt string, targetHash string) (bool, error) {
    saltBytes, err := hex.DecodeString(salt)
    if err != nil {
        return false, echo.NewHTTPError(http.StatusInternalServerError, "Error verifying password")
    }
    saltedPwd := append([]byte(passwordText), saltBytes...)
    pwdHash, err := sha256Hash(saltedPwd)
    pwd := hex.EncodeToString(pwdHash)

    return pwd == targetHash, nil
}

