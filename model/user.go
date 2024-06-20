package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"github.com/DavidTan0527/EC-admin-dashboard/auth"
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
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    coll := handler.HandlerConns.Db.Collection("User")

    user := new(User)
    if err := coll.FindOne(context.Background(), bson.M{"username": body.Username}).Decode(user); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username or password incorrect" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    correct, err := verifyPassword(body.Password, user.Salt, user.Password)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }
    if !correct {
        return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username or password incorrect" })
    }

    claims := auth.NewJwtClaims()
    claims.UserId = user.Id.Hex()
    claims.IsSuper = user.IsSuper
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims) 

    jwtKey, err := hex.DecodeString(os.Getenv("JWT_SECRET"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error generating JWT token" })
    }

    t, err := token.SignedString(jwtKey)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error generating JWT token" })
    }

    c.Logger().Info("User " + user.Username + " logged in")

    c.SetCookie(&http.Cookie{
        Name: "ec-t",
        Value: t,
        Expires: time.Now().Add(auth.TokenValidity),
    })

    return c.JSON(http.StatusOK, HttpResponseBody{
        Success: true,
        Message: "Login successful",
        Data: t,
    })
}

type CreateUserBody struct {
    Username string `json:"username" validate:"required"`
    Password string `json:"password" validate:"required"`
    IsSuper  bool   `json:"is_super" validate:"boolean"`
}

func (handler *UserHandler) CreateUser(c echo.Context) error {
    body := new(CreateUserBody)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    claims := GetJwtClaims(c)
    if body.IsSuper && !claims.IsSuper {
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "Only super users can create super users" })
    }

    user := new(User)
    user.Id = primitive.NewObjectID()
    user.Username = body.Username
    user.IsSuper = body.IsSuper

    password, salt, err := generateSaltAndPasswordHash(body.Password)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }
    user.Password = password
    user.Salt = salt

    coll := handler.HandlerConns.Db.Collection("User")
    if err := coll.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(new(User)); err == nil {
        return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Username exists" })
    } else if err != mongo.ErrNoDocuments {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    c.Logger().Info("Creating user " + user.Username + " with password hash " + user.Password)

    if res, err := coll.InsertOne(context.Background(), user); err != nil {
        c.Logger().Info(res)
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Cannot save user into DB" })
    }
    
    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "User " + user.Username + " created"})
}

func (handler *UserHandler) GetUser(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    claims := GetJwtClaims(c)
    if !claims.IsSuper && id.Hex() != claims.UserId {
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
        return c.JSON(http.StatusUnauthorized, HttpResponseBody{ Success: false, Message: "Non-super users cannot get all users"})
    }

    ctx := context.Background()

    coll := handler.HandlerConns.Db.Collection("User")
    cur, err := coll.Find(ctx, bson.M{})
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    data := make([]User, 0)
    err = cur.All(ctx, &data)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error reading result" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Data: data })
}

type UpdateUserPasswordBody struct {
    OldPassword string `json:"old_password" validate:"required"`
    NewPassword string `json:"new_password" validate:"required"`
}

func (handler *UserHandler) UpdateUserPassword(c echo.Context) error {
    body := new(UpdateUserPasswordBody)
    if err := GetRequestBody(c, body); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    claims := GetJwtClaims(c)
    id, err := primitive.ObjectIDFromHex(claims.UserId)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection("User")

    user := new(User)
    if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(user); err != nil {
        if err == mongo.ErrNoDocuments {
            return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "User not found" })
        }
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    correct, err := verifyPassword(body.OldPassword, user.Salt, user.Password)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }
    if !correct {
        return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Incorrect old password" })
    }

    password, salt, err := generateSaltAndPasswordHash(body.NewPassword)
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }
    user.Password = password
    user.Salt = salt

    if result, err := coll.ReplaceOne(ctx, bson.M{"_id": id}, user); err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    } else if result.MatchedCount == 0 {
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error updating password into DB" })
    }

    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "Password changed successfully" })
}

func (handler *UserHandler) DeleteUser(c echo.Context) error {
    id, err := primitive.ObjectIDFromHex(c.Param("id"))
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Server error" })
    }

    claims := GetJwtClaims(c)
    if claims.UserId == id.Hex() {
        return c.JSON(http.StatusOK, HttpResponseBody{ Success: false, Message: "Cannot delete yourself" })
    }

    ctx := context.Background()
    coll := handler.HandlerConns.Db.Collection("User")

    _, err = coll.DeleteOne(ctx, bson.M{"_id": id })
    if err != nil {
        c.Logger().Error(err)
        return c.JSON(http.StatusInternalServerError, HttpResponseBody{ Success: false, Message: "Error deleting user" })
    }

    c.Logger().Info("User with ID " + id.Hex() + " deleted")

    return c.JSON(http.StatusOK, HttpResponseBody{ Success: true, Message: "Successfully deleted user" })
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

