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

func (router *URLShortenerRouter) getAuthMiddleware(h http.Handler) http.Handler {
	authLogger := router.requestLogger.Named("Authentification")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := r.Cookie("shortener_token")
		var userID uint64
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
			userID = claims.UserID
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

			userID = claims.UserID
		}

		ctx := context.WithValue(r.Context(), "user_id", userID)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (router *URLShortenerRouter) buildJWTString(ctx context.Context) (model.Claims, string, error) {

	userID, err := router.service.GetStorage().CreateNewUser(ctx)
	if err != nil {
		return model.Claims{}, "", err
	}

	claims := model.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TOKEN_EXP)),
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(router.authKey)
	if err != nil {
		return model.Claims{}, "", err
	}

	return claims, tokenString, nil
}

func (router *URLShortenerRouter) parseJWTString(tokenString string) (model.Claims, error) {
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

func getUserIDFromCtx(ctx context.Context) (uint64, error) {
	rawuserID := ctx.Value("user_id")

	if userID, ok := rawuserID.(uint64); ok {
		return userID, nil
	} else {
		return 0, fmt.Errorf("can't get user id from context")
	}
}
