package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/apnaumov/url-shortener.git/internal/model"
	"github.com/golang-jwt/jwt/v4"
)

const TOKEN_EXP = time.Hour * 24 * 7

func (router *UrlShortenerRouter) getAuthMiddleware(h http.Handler) http.Handler {
	authLogger := router.requestLogger.Named("Authentification")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := r.Cookie("shortener_token")
		var userId uint64
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				authLogger.Error(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			claims, token, err := router.buildJWTString(r.Context())
			if err != nil {
				authLogger.Error(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			userId = claims.UserID
			http.SetCookie(w, &http.Cookie{Name: "shortener_token", Value: token, HttpOnly: true})
		} else {
			claims, err := router.parseJWTString(token.Value)

			if err != nil {
				authLogger.Error(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if claims.UserID == 0 {
				authLogger.Error("User id is empty")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			userId = claims.UserID
		}

		ctx := context.WithValue(r.Context(), "user_id", userId)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (router *UrlShortenerRouter) buildJWTString(ctx context.Context) (model.Claims, string, error) {

	userId, err := router.service.GetStorage().CreateNewUser(ctx)
	if err != nil {
		return model.Claims{}, "", err
	}

	claims := model.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TOKEN_EXP)),
		},
		UserID: userId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(router.authKey)
	if err != nil {
		return model.Claims{}, "", err
	}

	return claims, tokenString, nil
}

func (router *UrlShortenerRouter) parseJWTString(tokenString string) (model.Claims, error) {
	claims := model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(router.authKey), nil
		})
	if err != nil {
		return model.Claims{}, err
	}

	if !token.Valid {
		return model.Claims{}, fmt.Errorf("token is not valid. Token: %s", tokenString)
	}

	return claims, nil
}

func getUserIdFromCtx(ctx context.Context) (uint64, error) {
	rawUserId := ctx.Value("user_id")

	if userId, ok := rawUserId.(uint64); ok {
		return userId, nil
	} else {
		return 0, fmt.Errorf("can't get user id from context")
	}
}
