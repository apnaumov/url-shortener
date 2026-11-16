package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/apnaumov/url-shortener.git/internal/repository"
)

// userIDKey — ключ ID пользователя в контексте запроса.
type userIDKey struct{}

// tokenExp — срок жизни JWT-токена.
const tokenExp = time.Hour * 24 * 7

// getAuthMiddleware аутентифицирует пользователя по cookie shortener_token.
// Если cookie нет или токен невалиден, создаёт нового пользователя и выдаёт новый токен.
// ID пользователя кладёт в контекст запроса.
func (router *URLShortenerRouter) getAuthMiddleware(h http.Handler) http.Handler {
	authLogger := router.requestLogger.Named("Authentification")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		setNewCookie := func() (uint64, error) {
			claims, token, err := router.jwtClient.BuildToken(r.Context())
			if err != nil {
				return 0, err
			}

			http.SetCookie(w, &http.Cookie{Name: "shortener_token", Value: token, HttpOnly: true})
			return claims.UserID, nil
		}

		token, err := r.Cookie("shortener_token")
		var userID uint64
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				authLogger.Error(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			newUserID, err := setNewCookie()
			if err != nil {
				authLogger.Error(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			userID = newUserID
		} else {
			claims, err := router.jwtClient.ParseToken(token.Value)

			if err != nil {
				newUserID, err := setNewCookie()
				if err != nil {
					authLogger.Error(err.Error())
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				userID = newUserID
			} else {
				userID = claims.UserID

				if userID == 0 {
					authLogger.Error("User id is empty")
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}
			}
		}

		ctx := context.WithValue(r.Context(), userIDKey{}, userID)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// jwtClient создаёт и проверяет JWT-токены пользователей.
type jwtClient struct {
	storage repository.URLStorage
	authKey []byte
}

// NewJWTClient создаёт клиент, подписывающий токены ключом authKey.
// storage используется для создания новых пользователей.
func NewJWTClient(storage repository.URLStorage, authKey []byte) *jwtClient {
	return &jwtClient{
		storage: storage,
		authKey: authKey,
	}
}

// BuildToken создаёт нового пользователя в хранилище и возвращает его claims
// и подписанный токен.
func (jwtClient *jwtClient) BuildToken(ctx context.Context) (model.Claims, string, error) {
	userID, err := jwtClient.storage.CreateNewUser(ctx)
	if err != nil {
		return model.Claims{}, "", err
	}

	claims := model.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExp)),
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(jwtClient.authKey)
	if err != nil {
		return model.Claims{}, "", err
	}

	return claims, tokenString, nil
}

// ParseToken проверяет подпись и срок действия токена и возвращает его claims.
func (jwtClient *jwtClient) ParseToken(tokenString string) (model.Claims, error) {
	claims := model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtClient.authKey), nil
		})
	if err != nil {
		return model.Claims{}, err
	}

	if !token.Valid {
		return model.Claims{}, fmt.Errorf("token is not valid. Token: %s", tokenString)
	}

	return claims, nil
}

// getUserIDFromCtx достаёт ID пользователя, положенный auth-мидлварью.
func getUserIDFromCtx(ctx context.Context) (uint64, error) {
	rawUserID := ctx.Value(userIDKey{})

	if userID, ok := rawUserID.(uint64); ok {
		return userID, nil
	} else {
		return 0, fmt.Errorf("can't get user id from context")
	}
}
