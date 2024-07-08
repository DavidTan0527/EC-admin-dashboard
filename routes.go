package main

import (
	"context"
	"encoding/hex"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/DavidTan0527/EC-admin-dashboard/auth"
	"github.com/DavidTan0527/EC-admin-dashboard/model"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

// Middlewares for handlers
type Middlewares struct {
    Jwt     echo.MiddlewareFunc
    IsSuper echo.MiddlewareFunc
}

func initRoutes(conns *model.HandlerConns) *echo.Echo {
    e := echo.New()
    setupMiddlewares(e)

    middlewares := initCustomMiddlewares()

	e.GET("/ping", model.Ping)
    e.GET("/checkToken", model.Ping, middlewares.Jwt)
    initUserRoutes(e, conns, middlewares)
    initPermRoutes(e, conns, middlewares)
    initTableRoutes(e, conns, middlewares)
    initChartRoutes(e, conns, middlewares)
    initChartViewRoutes(e, conns, middlewares)

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		if err := e.Start(":" + os.Getenv("SERVER_PORT")); err != nil && err != http.ErrServerClosed {
            e.Logger.Debug(err)
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}

    return e
}

func initUserRoutes(e *echo.Echo, httpHandler *model.HandlerConns, middlewares *Middlewares) {
    handler := model.UserHandler{ HandlerConns: httpHandler }
    e.POST("/register", handler.CreateUser, middlewares.Jwt)
    e.POST("/login", handler.LoginUser)
    e.POST("/change_pwd", handler.UpdateUserPassword, middlewares.Jwt)

    e.GET("/user/:id", handler.GetUser, middlewares.Jwt)
    e.DELETE("/user/:id", handler.DeleteUser, middlewares.Jwt, middlewares.IsSuper)
    e.GET("/users", handler.GetAllUsers, middlewares.Jwt, middlewares.IsSuper)
}

func initPermRoutes(e *echo.Echo, httpHandler *model.HandlerConns, middlewares *Middlewares) {
    handler := model.PermissionHandler{ HandlerConns: httpHandler }
    e.GET("/permission/:key", handler.CheckPerm, middlewares.Jwt)
    e.GET("/permission/:key/list", handler.GetPermUserList, middlewares.Jwt, middlewares.IsSuper)
    e.POST("/permission", handler.SetPerm, middlewares.Jwt, middlewares.IsSuper)
    e.DELETE("/permission", handler.RemovePerm, middlewares.Jwt, middlewares.IsSuper)

    e.GET("/permission_keys", handler.GetAllPermKey, middlewares.Jwt)
}

func initTableRoutes(e *echo.Echo, httpHandler *model.HandlerConns, middlewares *Middlewares) {
    handler := model.TableHandler{ HandlerConns: httpHandler }
    e.GET("/table", handler.GetTableList, middlewares.Jwt)
    e.POST("/table", handler.CreateTable, middlewares.Jwt)
    e.GET("/table/:id", handler.GetTableFull, middlewares.Jwt)
    e.GET("/table/:id/:year/:month", handler.GetTable, middlewares.Jwt)
    e.POST("/table/:id/:year/:month", handler.EditTableData, middlewares.Jwt)
    e.PUT("/table/:id", handler.EditTableMetadata, middlewares.Jwt)
    e.DELETE("/table/:id", handler.DeleteTable, middlewares.Jwt)

    e.PUT("/table/sort/:id", handler.EditTableSort, middlewares.Jwt)

    e.GET("/table/schema", handler.GetAllTableSchema, middlewares.Jwt)
    e.GET("/table/schema/:id", handler.GetTableSchema, middlewares.Jwt)
}

func initChartRoutes(e *echo.Echo, httpHandler *model.HandlerConns, middlewares *Middlewares) {
    handler := model.ChartHandler{ HandlerConns: httpHandler }
    e.GET("/chart", handler.GetAllChart, middlewares.Jwt)
    e.GET("/chart/:id", handler.GetChart, middlewares.Jwt)
    e.POST("/chart", handler.CreateChart, middlewares.Jwt)
    e.PUT("/chart", handler.EditChart, middlewares.Jwt)
    e.DELETE("/chart/:id", handler.DeleteChart, middlewares.Jwt)
}

func initChartViewRoutes(e *echo.Echo, httpHandler *model.HandlerConns, middlewares *Middlewares) {
    handler := model.ChartViewHandler{ HandlerConns: httpHandler }
    e.GET("/chart_view", handler.GetChartViewList, middlewares.Jwt)
    e.GET("/chart_view/:id", handler.LoadChartView, middlewares.Jwt)
    e.POST("/chart_view", handler.CreateChartView, middlewares.Jwt)
    e.PUT("/chart_view", handler.EditChartView, middlewares.Jwt)
    e.DELETE("/chart_view/:id", handler.DeleteChartView, middlewares.Jwt)
}

func initCustomMiddlewares() *Middlewares {
    jwtKey, err := hex.DecodeString(os.Getenv("JWT_SECRET"))
    if err != nil {
        panic("Invalid JWT secret")
    }

    return &Middlewares{
        Jwt: echojwt.WithConfig(echojwt.Config{
            NewClaimsFunc: func(c echo.Context) jwt.Claims {
                return new(auth.JwtClaims)
            },
            SigningKey: jwtKey,
        }),

        IsSuper: func (next echo.HandlerFunc) echo.HandlerFunc {
            return func (c echo.Context) error {
                claims := model.GetJwtClaims(c)
                if !claims.IsSuper {
                    c.Error(echo.NewHTTPError(http.StatusUnauthorized, "Not super user"))
                    return nil
                }

                if err := next(c); err != nil {
                    c.Error(err)
                }

                return nil
            }
        },
    }
}

func setupMiddlewares(e *echo.Echo) {
    e.Use(middleware.CORS())
    e.Validator = &RequestValidator{ validator: validator.New() }

    e.Logger.SetLevel(log.DEBUG)
    if l, ok := e.Logger.(*log.Logger); ok {
        l.SetHeader("${time_rfc3339} [${level}] ${short_file}:${line}\n")
    }

    e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
        Format: "method=${method}, uri=${uri} (${latency_human}), status=${status}\nerror: ${error}\n",
    }))
}

