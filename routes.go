package main

import (
	"context"
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

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

    middlewares := initMiddlewares()
    initUserRoutes(e, conns, middlewares)
    initPermRoutes(e, conns, middlewares)

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		if err := e.Start("localhost:" + os.Getenv("SERVER_PORT")); err != nil && err != http.ErrServerClosed {
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
    e.POST("/changePwd", handler.UpdateUserPassword, middlewares.Jwt)

    e.GET("/user/:id", handler.GetUser, middlewares.Jwt)
    e.GET("/users", handler.GetAllUsers, middlewares.Jwt, middlewares.IsSuper)
}

func initPermRoutes(e *echo.Echo, httpHandler *model.HandlerConns, middlewares *Middlewares) {
    handler := model.PermissionHandler{ HandlerConns: httpHandler }
    e.GET("/permission", handler.CheckPerm, middlewares.Jwt)
    e.POST("/permission", handler.SetPerm, middlewares.Jwt, middlewares.IsSuper)
}

func initMiddlewares() *Middlewares {
    return &Middlewares{
        Jwt: echojwt.WithConfig(echojwt.Config{
            NewClaimsFunc: func(c echo.Context) jwt.Claims {
                return new(auth.JwtClaims)
            },
            SigningKey: []byte(os.Getenv("JWT_SECRET")),
        }),

        IsSuper: func (next echo.HandlerFunc) echo.HandlerFunc {
            return func (c echo.Context) error {
                claims := model.GetJwtClaims(c)
                if !claims.IsSuper {
                    c.Error(echo.NewHTTPError(http.StatusUnauthorized, "Not super user"))
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
    e.Logger.SetLevel(log.DEBUG)
    if l, ok := e.Logger.(*log.Logger); ok {
        l.SetHeader("${time_rfc3339} [${level}] ${short_file}:${line}\n")
    }

    e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
        Format: "method=${method}, uri=${uri} (${latency_human}), status=${status}\nerror: ${error}\n",
    }))

    e.Validator = &RequestValidator{ validator: validator.New() }
}

